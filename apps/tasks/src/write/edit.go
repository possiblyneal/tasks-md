package write

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/history"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/repos"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// Edit is a change to a Task's attributes as an edit asks for it. A nil field
// is left as it is and an empty one takes the attribute off; Tags and
// BlockedBy replace the whole set. State is not here: a Task changes State
// through Move, which holds the rules for it. Version is the Task's tree's
// version as the caller last read it, or empty for a caller that showed
// nothing and leans on the lock alone.
type Edit struct {
	Version     string    `json:"version,omitempty"`
	Title       *string   `json:"title,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	Description *string   `json:"description,omitempty"`
	Why         *string   `json:"why,omitempty"`
	Acceptance  *string   `json:"acceptance,omitempty"`
	Deadline    *string   `json:"deadline,omitempty"`
	Priority    *string   `json:"priority,omitempty"`
	Impact      *string   `json:"impact,omitempty"`
	Estimate    *string   `json:"estimate,omitempty"`
	Color       *string   `json:"color,omitempty"`
	BlockedBy   *[]string `json:"blockedBy,omitempty"`
	Reason      *string   `json:"reason,omitempty"`
	Until       *string   `json:"until,omitempty"`
	Attach      []string  `json:"attach,omitempty"`
	Detach      []string  `json:"detach,omitempty"`
}

// EditTask changes the Task with the id as the Edit asks. Labels it does not
// name, the tracker's or a person's own, are kept where they are.
func EditTask(dir, actor, id string, e Edit) error {
	return history.Write(dir, actor, func(before string) (string, string, error) {
		f, path, err := open(before, id, e.Version)
		if err != nil {
			return "", "", err
		}
		t := path.task()
		// The fields given are checked as an add would check them, against
		// the Task's own State, so an edit holds the rules an add does.
		n := New{Title: t.Title, State: string(t.State), Attach: e.Attach}
		for _, field := range []struct {
			to   *string
			from *string
		}{
			{&n.Title, e.Title}, {&n.Description, e.Description}, {&n.Why, e.Why}, {&n.Acceptance, e.Acceptance},
			{&n.Deadline, e.Deadline}, {&n.Priority, e.Priority}, {&n.Impact, e.Impact}, {&n.Estimate, e.Estimate},
			{&n.Color, e.Color}, {&n.Reason, e.Reason}, {&n.Until, e.Until},
		} {
			if field.from != nil {
				*field.to = *field.from
			}
		}
		if e.Tags != nil {
			n.Tags = given(*e.Tags)
		}
		if e.BlockedBy != nil {
			n.BlockedBy = given(*e.BlockedBy)
		}
		if _, err := n.check(); err != nil {
			return "", "", err
		}
		if err := blockers(n.BlockedBy, every(f.Tasks), id); err != nil {
			return "", "", err
		}

		t.Title = n.Title
		if e.Tags != nil {
			t.Tags = n.Tags
		}
		if e.Description != nil {
			t.Description = n.Description
		}
		for _, a := range []struct {
			label string
			given bool
			value string
		}{
			{"deadline", e.Deadline != nil, n.Deadline},
			{"priority", e.Priority != nil, n.Priority},
			{"impact", e.Impact != nil, n.Impact},
			{"estimate", e.Estimate != nil, n.Estimate},
			{"color", e.Color != nil, n.Color},
			{"why", e.Why != nil, n.Why},
			{"acceptance", e.Acceptance != nil, n.Acceptance},
			{"blocked by", e.BlockedBy != nil, strings.Join(n.BlockedBy, ", ")},
			{"until", e.Until != nil, n.Until},
			{"reason", e.Reason != nil, n.Reason},
		} {
			if a.given {
				set(t, a.label, a.value)
			}
		}
		attach(t, e.Attach, e.Detach)
		if _, err := fill(dir, f); err != nil {
			return "", "", err
		}
		return taskfile.Write(f), "edit ^" + id + " " + t.Title, nil
	})
}

// Delete takes the Task with the id out of the file, its Subtasks with it.
// The tasks history keeps the block. A blocker the delete took is taken off
// every Task it blocked, since a blocker naming nothing blocks nothing.
func Delete(dir, actor, id, version string) error {
	return history.Write(dir, actor, func(before string) (string, string, error) {
		f, path, err := open(before, id, version)
		if err != nil {
			return "", "", err
		}
		at := path.last()
		gone := every([]*taskfile.Task{at.task})
		*at.siblings = slices.DeleteFunc(*at.siblings, func(t *taskfile.Task) bool { return t == at.task })

		was := make([]bool, len(path)-1)
		for i, above := range path[:len(path)-1] {
			was[i] = above.task.State.Ended()
		}
		f.Settle()
		for i := len(path) - 2; i >= 0; i-- {
			path[i].arrange(was[i])
		}
		walk(f.Tasks, func(t *taskfile.Task) {
			kept := slices.DeleteFunc(t.BlockedBy(), func(id string) bool { _, ok := gone[id]; return ok })
			if len(kept) != len(t.BlockedBy()) {
				set(t, "blocked by", strings.Join(kept, ", "))
			}
		})
		if _, err := fill(dir, f); err != nil {
			return "", "", err
		}
		return taskfile.Write(f), "delete ^" + id + " " + at.task.Title, nil
	})
}

// RenameTag gives every Task carrying the Tag from the Tag to instead, as one
// commit in each Repo that has one, and says which Repos it wrote. A Task carrying both keeps one. Versions
// are the trees the caller read, and the rename is refused if any tree it
// would touch is not among them; nil is a caller that showed nothing. Every
// Repo is checked before any is written, so a stale one refuses the whole
// rename rather than half of it; a write racing the rename between the check
// and the lock can still stop it partway.
func RenameTag(dirs []string, actor, from, to string, versions []string) ([]string, error) {
	for _, tag := range []string{from, to} {
		if !tagForm.MatchString(tag) {
			return nil, Invalid{fmt.Errorf("%q is not a Tag: one word, with no #, | or space", tag)}
		}
	}
	rename := func(dir string) history.Change {
		return func(before string) (string, string, error) {
			f, err := parse(before)
			if err != nil {
				return "", "", err
			}
			for _, top := range f.Tasks {
				if carries([]*taskfile.Task{top}, from) && versions != nil && !slices.Contains(versions, taskfile.Version(top)) {
					return "", "", Refused{fmt.Sprintf("^%s's tree has changed since it was read, so #%s was not renamed; read it again", top.ID, from)}
				}
			}
			walk(f.Tasks, func(t *taskfile.Task) {
				if !slices.Contains(t.Tags, from) {
					return
				}
				var tags []string
				for _, tag := range t.Tags {
					if tag == from {
						tag = to
					}
					if !slices.Contains(tags, tag) {
						tags = append(tags, tag)
					}
				}
				t.Tags = tags
			})
			if _, err := fill(dir, f); err != nil {
				return "", "", err
			}
			return taskfile.Write(f), "rename #" + from + " to #" + to, nil
		}
	}
	// A Repo carrying no Task with the Tag is left alone, broken or not.
	var carrying []string
	for _, dir := range dirs {
		text, err := os.ReadFile(filepath.Join(dir, repos.FileName))
		if err != nil {
			return nil, err
		}
		if f, _ := taskfile.Parse(string(text)); !carries(f.Tasks, from) {
			continue
		}
		if _, _, err := rename(dir)(string(text)); err != nil {
			return nil, err
		}
		carrying = append(carrying, dir)
	}
	for i, dir := range carrying {
		if err := history.Write(dir, actor, rename(dir)); err != nil {
			return carrying[:i], err
		}
	}
	return carrying, nil
}

// open reads the file an edit or a delete rewrites and finds the Task, refusing
// the write when the caller read the Task's tree as it no longer stands.
func open(before, id, version string) (*taskfile.File, trail, error) {
	f, err := parse(before)
	if err != nil {
		return nil, nil, err
	}
	path := find(f, id)
	if path == nil {
		return nil, nil, NoTask(id)
	}
	if version != "" && taskfile.Version(path[0].task) != version {
		return nil, nil, Refused{fmt.Sprintf("^%s's tree has changed since it was read; read it again", id)}
	}
	return f, path, nil
}

// attach writes an `attachment:` line for each pointer the Task does not hold
// yet, after taking off each one detached.
func attach(t *taskfile.Task, add, remove []string) {
	t.Attrs = slices.DeleteFunc(t.Attrs, func(a taskfile.Attr) bool {
		return a.Label == "attachment" && slices.Contains(remove, a.Value)
	})
	for _, pointer := range add {
		if !slices.Contains(t.Attrs, taskfile.Attr{Label: "attachment", Value: pointer}) {
			t.Attrs = append(t.Attrs, taskfile.Attr{Label: "attachment", Value: pointer})
		}
	}
}

// blockers refuses a blocker the Repo holds no Task under, and a Task waiting
// on itself.
func blockers(ids []string, in map[string]struct{}, self string) error {
	for _, blocker := range ids {
		if blocker == self {
			return Invalid{fmt.Errorf("^%s cannot wait on itself", self)}
		}
		if _, ok := in[blocker]; !ok {
			return Invalid{fmt.Errorf("blocked by %s, which is no Task in this Repo", blocker)}
		}
	}
	return nil
}

// fill gives a Task typed by hand without an id or a created date the ones an
// add would have, so a write leaves no Task the file's rules flag for it. It
// answers the ids the Repo has spent, those it gave included.
func fill(dir string, f *taskfile.File) (map[string]struct{}, error) {
	ids, err := spent(dir, f)
	if err != nil {
		return nil, err
	}
	walk(f.Tasks, func(t *taskfile.Task) {
		if t.ID == "" {
			t.ID = fresh(ids)
			ids[t.ID] = struct{}{}
		}
		if t.Created == "" {
			t.Created = today()
		}
		// A hand-ticked end is dated when it is found, so it is counted.
		if t.State.Ended() && t.Attr("ended") == "" {
			set(t, "ended", today())
		}
	})
	return ids, nil
}

// given is a set as a caller typed it, without the empty entries a cleared
// flag or field leaves.
func given(set []string) []string {
	return slices.DeleteFunc(slices.Clone(set), func(s string) bool { return s == "" })
}

// carries says whether any Task in the trees carries the Tag.
func carries(tasks []*taskfile.Task, tag string) bool {
	found := false
	walk(tasks, func(t *taskfile.Task) { found = found || slices.Contains(t.Tags, tag) })
	return found
}

func walk(tasks []*taskfile.Task, visit func(*taskfile.Task)) {
	for _, t := range tasks {
		visit(t)
		walk(t.Subtasks, visit)
	}
}
