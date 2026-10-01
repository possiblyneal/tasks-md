package write

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/history"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/schedule"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// Server is the Actor of the writes nobody asked for: the pass's.
const Server = "task server"

// Pass brings each Repo's tasks.md up to today before anything reads it or
// writes to it. A Deferred Task whose until date has come moves to Backlog,
// and an ended Task still carrying a Series has its next Occurrence written
// and the rule moved onto it. Each Repo with anything due gets one commit,
// made as the task server.
//
// Nothing here fails the call that asked for the pass. A Repo refusing writes
// is passed over and stays flagged on every read, and a file a rewrite would
// drop a line of stays among its problems, until a person clears it.
func Pass(today time.Time, dirs ...string) {
	for _, dir := range dirs {
		// The file is read without the lock first, so a pass with nothing to
		// do costs a read rather than a commit's worth of git.
		text, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
		if err != nil {
			continue
		}
		if f, _ := taskfile.Parse(string(text)); len(due(f, today)) == 0 {
			continue
		}
		_ = history.Write(dir, Server, func(before string) (string, string, error) {
			f, err := parse(before)
			if err != nil {
				return "", "", err
			}
			done := due(f, today)
			return taskfile.Write(f), strings.Join(done, ", "), nil
		})
	}
}

// due does to f what today makes due and says what it did, one phrase each.
func due(f *taskfile.File, today time.Time) []string {
	date := today.Format(time.DateOnly)
	var done []string
	var ended [][]at
	walk(f, func(path []at) {
		t := path[len(path)-1].task
		if len(t.Subtasks) == 0 && t.State == taskfile.Deferred && t.Attr("until") != "" && t.Attr("until") <= date {
			t.State = taskfile.Backlog
			set(t, "until", "")
			set(t, "reason", "")
			done = append(done, "wake ^"+t.ID+" to Backlog")
		}
		if t.State.Ended() && t.Attr("series") != "" {
			ended = append(ended, path)
		}
	})
	for _, path := range ended {
		if phrase := repeat(f, path, today); phrase != "" {
			done = append(done, phrase)
		}
	}
	f.Settle()
	return done
}

// repeat writes the next Occurrence after the ended Task at the end of path
// and moves the rule onto it, or takes the rule off when it has run out. The
// next is due on the first date the rule produces after both today and the
// ended one's Deadline, so missed dates are not made up and an early finish
// does not repeat its own date.
func repeat(f *taskfile.File, path []at, today time.Time) string {
	old := path[len(path)-1]
	rule, err := schedule.Parse(old.task.Attr("series"))
	if err != nil {
		// A rule nobody can read is left where it is for a person to fix.
		return ""
	}
	after := day(today)
	if deadline, err := time.ParseInLocation(time.DateOnly, old.task.Attr("deadline"), time.Local); err == nil && deadline.After(after) {
		after = deadline
	}
	next, ok := rule.Next(after.AddDate(0, 0, 1))
	if !ok {
		set(old.task, "series", "")
		return "end the series of ^" + old.task.ID
	}

	ids := every(f.Tasks)
	named := map[*taskfile.Task]string{}
	renamed := map[string]string{}
	var name func(*taskfile.Task)
	name = func(t *taskfile.Task) {
		id := fresh(ids)
		ids[id] = struct{}{}
		named[t] = id
		if t.ID != "" {
			renamed[t.ID] = id
		}
		for _, s := range t.Subtasks {
			name(s)
		}
	}
	name(old.task)
	created := today.Format(time.DateOnly)
	n := reset(old.task, named, renamed, created)
	set(n, "deadline", next.Format(time.DateOnly))
	set(n, "series", rule.String())
	set(old.task, "series", "")

	was := make([]bool, len(path)-1)
	for i, a := range path[:len(path)-1] {
		was[i] = a.task.State.Ended()
	}
	*old.siblings = slices.Insert(*old.siblings, firstEnded(*old.siblings), n)
	f.Settle()
	for i := len(path) - 2; i >= 0; i-- {
		path[i].arrange(was[i])
	}
	return "next ^" + n.ID + " after ^" + old.task.ID
}

// reset is a copy of t and its Subtasks as a fresh Occurrence: new ids,
// created today, every one in Backlog, and nothing an ended or Deferred Task
// carries. A blocker inside the copied tree is the copy of that blocker.
func reset(t *taskfile.Task, named map[*taskfile.Task]string, renamed map[string]string, created string) *taskfile.Task {
	n := &taskfile.Task{
		Title:       t.Title,
		State:       taskfile.Backlog,
		Tags:        slices.Clone(t.Tags),
		ID:          named[t],
		Created:     created,
		Description: t.Description,
	}
	for _, a := range t.Attrs {
		switch a.Label {
		case "ended", "reason", "until":
			continue
		case "blocked by":
			ids := t.BlockedBy()
			for i, id := range ids {
				if to, ok := renamed[id]; ok {
					ids[i] = to
				}
			}
			a.Value = strings.Join(ids, ", ")
		}
		n.Attrs = append(n.Attrs, a)
	}
	for _, s := range t.Subtasks {
		n.Subtasks = append(n.Subtasks, reset(s, named, renamed, created))
	}
	return n
}

// walk calls visit with the way down to every Task in f, outermost first, in
// file order.
func walk(f *taskfile.File, visit func([]at)) {
	var down func(list *[]*taskfile.Task, above []at)
	down = func(list *[]*taskfile.Task, above []at) {
		for _, t := range *list {
			path := append(slices.Clip(above), at{t, list})
			visit(path)
			down(&t.Subtasks, path)
		}
	}
	down(&f.Tasks, nil)
}
