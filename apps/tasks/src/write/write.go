// Package write is the one write path onto a Repo's TASKS.md. Each verb reads
// the file, applies Tracking's rules to the Task tree, writes it back through
// taskfile and hands the text to history, which commits and pushes it. `tasks
// add`, `tasks move` and their API routes all call here, so a person and an
// Agent are held to the same rules.
//
// The rules applied here: a Task added without a State is in Inbox; a parent's
// State is worked out, so a parent is never moved; moving a Task already in
// Doing to Doing is refused, because Doing is how a Task is taken; a Reason is
// held only while Deferred or Declined; `ended:` is written when a Task ends
// and cleared when it reopens; and ended Tasks sink to the bottom of their
// level in the order they ended.
package write

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/history"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// Refused is a write the rules turn away: exit 3 at a terminal and 409 over
// the API. The caller reads again and decides; nothing was written.
type Refused struct{ Reason string }

func (r Refused) Error() string { return r.Reason }

// IsRefused says whether err turned a write away, whether a rule here did or
// the Repo is refusing writes.
func IsRefused(err error) bool {
	var refused Refused
	var unready history.Unready
	return errors.As(err, &refused) || errors.As(err, &unready)
}

// Invalid is a value the caller can fix by asking differently: exit 2 and 400.
type Invalid struct{ error }

func (i Invalid) Unwrap() error { return i.error }

// NoTask is an id the Repo holds no Task under: exit 1 and 404.
type NoTask string

func (n NoTask) Error() string { return fmt.Sprintf("no Task has the id %s", string(n)) }

// New is a Task as an add asks for it. An empty field is left off the Task.
// It is the API's body as well, so the CLI's flags and the JSON name the same
// attributes.
type New struct {
	Title       string   `json:"title"`
	State       string   `json:"state,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Description string   `json:"description,omitempty"`
	Why         string   `json:"why,omitempty"`
	Acceptance  string   `json:"acceptance,omitempty"`
	Deadline    string   `json:"deadline,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Impact      string   `json:"impact,omitempty"`
	Estimate    string   `json:"estimate,omitempty"`
	Color       string   `json:"color,omitempty"`
	BlockedBy   []string `json:"blockedBy,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Until       string   `json:"until,omitempty"`
	Attach      []string `json:"attach,omitempty"`
}

var (
	levels    = []string{"low", "med", "high"}
	estimates = []string{"small", "medium", "large"}
	colors    = []string{"red", "orange", "yellow", "green", "cyan", "blue", "violet", "magenta", "brown"}
	tagForm   = regexp.MustCompile(`^[^\s#|]+$`)
)

// Add writes a new top-level Task into dir's TASKS.md and names it. It is
// never stale: it touches no Task already there.
func Add(dir, actor string, n New) (string, error) {
	state, err := n.check()
	if err != nil {
		return "", err
	}
	var id string
	err = history.Write(dir, actor, func(before string) (string, string, error) {
		f, err := parse(before)
		if err != nil {
			return "", "", err
		}
		fill(f)
		ids := every(f.Tasks)
		if err := blockers(n.BlockedBy, ids, ""); err != nil {
			return "", "", err
		}
		id = fresh(ids)
		place(&f.Tasks, n.task(state, id))
		return taskfile.Write(f), "add ^" + id + " " + n.Title, nil
	})
	return id, err
}

func (n New) check() (taskfile.State, error) {
	invalid := func(format string, args ...any) (taskfile.State, error) {
		return "", Invalid{fmt.Errorf(format, args...)}
	}
	if strings.TrimSpace(n.Title) == "" || strings.ContainsAny(n.Title, "\r\n") {
		return invalid("a Task needs a title of one line")
	}
	state := taskfile.State(n.State)
	if n.State == "" {
		state = taskfile.Inbox
	}
	if err := known(state); err != nil {
		return "", err
	}
	if err := reasonFits(n.Reason, state); err != nil {
		return "", err
	}
	if n.Until != "" && state != taskfile.Deferred {
		return invalid("until is held only while a Task is Deferred, not %s", title(state))
	}
	for _, tag := range n.Tags {
		if !tagForm.MatchString(tag) {
			return invalid("%q is not a Tag: one word, with no #, | or space", tag)
		}
	}
	for _, d := range []struct{ name, value string }{{"deadline", n.Deadline}, {"until", n.Until}} {
		if _, err := time.Parse(time.DateOnly, d.value); d.value != "" && err != nil {
			return invalid("%s %q is not a date: want YYYY-MM-DD", d.name, d.value)
		}
	}
	for _, c := range []struct {
		name, value string
		from        []string
	}{
		{"priority", n.Priority, levels},
		{"impact", n.Impact, levels},
		{"estimate", n.Estimate, estimates},
		{"color", n.Color, colors},
	} {
		if c.value != "" && !slices.Contains(c.from, c.value) {
			return invalid("%s %q is not one of %s", c.name, c.value, strings.Join(c.from, ", "))
		}
	}
	for _, field := range append([]string{n.Why, n.Acceptance, n.Reason}, n.Attach...) {
		if strings.ContainsAny(field, "\r\n") {
			return invalid("only the description may hold more than one line")
		}
	}
	return state, nil
}

// Move puts the Task with the id in another State.
func Move(dir, actor, id string, to taskfile.State, reason string) error {
	if err := known(to); err != nil {
		return err
	}
	if err := reasonFits(reason, to); err != nil {
		return err
	}
	return history.Write(dir, actor, func(before string) (string, string, error) {
		f, err := parse(before)
		if err != nil {
			return "", "", err
		}
		fill(f)
		path := find(f, id)
		if path == nil {
			return "", "", NoTask(id)
		}
		t := path[len(path)-1].task
		if len(t.Subtasks) > 0 {
			return "", "", Refused{fmt.Sprintf("^%s is a parent: its State is worked out from its Subtasks, so move one of them", id)}
		}
		if to == taskfile.Doing && t.State == taskfile.Doing {
			return "", "", Refused{fmt.Sprintf("^%s is already in Doing, so somebody has taken it", id)}
		}

		was := make([]bool, len(path))
		for i, at := range path {
			was[i] = at.task.State.Ended()
		}
		t.State = to
		set(t, "reason", reason)
		if to != taskfile.Deferred {
			set(t, "until", "")
		}
		f.Settle()
		// The moved Task first, then each parent its move ended or reopened.
		for i := len(path) - 1; i >= 0; i-- {
			path[i].arrange(was[i])
		}
		return taskfile.Write(f), "move ^" + id + " to " + title(to), nil
	})
}

// AddSubtasks writes Subtasks under the parent as one commit, in the order
// given, and gives back their ids. version is the parent's tree's as it was
// read, or empty to write whatever the tree is now.
func AddSubtasks(dir, actor, parent, version string, subtasks []New) ([]string, error) {
	if len(subtasks) == 0 {
		return nil, Invalid{errors.New("give at least one Subtask")}
	}
	states := make([]taskfile.State, len(subtasks))
	for i, n := range subtasks {
		state, err := n.check()
		if err != nil {
			return nil, err
		}
		states[i] = state
	}
	var made []string
	err := history.Write(dir, actor, func(before string) (string, string, error) {
		made = nil
		f, path, err := open(before, parent, version)
		if err != nil {
			return "", "", err
		}
		fill(f)
		ids := every(f.Tasks)
		under := path[len(path)-1].task
		was := make([]bool, len(path))
		for i, at := range path {
			was[i] = at.task.State.Ended()
		}
		for i, n := range subtasks {
			if err := blockers(n.BlockedBy, ids, ""); err != nil {
				return "", "", err
			}
			id := fresh(ids)
			ids[id] = struct{}{}
			made = append(made, id)
			place(&under.Subtasks, n.task(states[i], id))
		}
		// The parent is worked out afresh, so an ended one reopens and rises.
		f.Settle()
		for i := len(path) - 1; i >= 0; i-- {
			path[i].arrange(was[i])
		}
		return taskfile.Write(f), "add ^" + strings.Join(made, " ^") + " under ^" + parent, nil
	})
	if err != nil {
		return nil, err
	}
	return made, nil
}

// task is the new Task the fields describe.
func (n New) task(state taskfile.State, id string) *taskfile.Task {
	t := &taskfile.Task{Title: n.Title, State: state, Tags: n.Tags, ID: id, Created: today(), Description: n.Description}
	for _, a := range []taskfile.Attr{
		{Label: "deadline", Value: n.Deadline},
		{Label: "priority", Value: n.Priority},
		{Label: "impact", Value: n.Impact},
		{Label: "estimate", Value: n.Estimate},
		{Label: "color", Value: n.Color},
		{Label: "why", Value: n.Why},
		{Label: "acceptance", Value: n.Acceptance},
		{Label: "blocked by", Value: strings.Join(n.BlockedBy, ", ")},
		{Label: "until", Value: n.Until},
		{Label: "reason", Value: n.Reason},
	} {
		set(t, a.Label, a.Value)
	}
	attach(t, n.Attach, nil)
	return t
}

// place puts a new Task into its level: an ended one at the bottom, an open
// one above the first ended.
func place(list *[]*taskfile.Task, t *taskfile.Task) {
	if t.State.Ended() {
		set(t, "ended", today())
		*list = append(*list, t)
		return
	}
	*list = slices.Insert(*list, firstEnded(*list), t)
}

// at is one Task on the way down to another, with the list it sits in.
type at struct {
	task     *taskfile.Task
	siblings *[]*taskfile.Task
}

// arrange writes `ended:` and moves the Task within its level when it has
// ended or reopened: an ended Task goes to the bottom, after those that ended
// before it, and a reopened one to the bottom of the open ones.
func (a at) arrange(wasEnded bool) {
	ended := a.task.State.Ended()
	if ended == wasEnded {
		return
	}
	list := slices.DeleteFunc(*a.siblings, func(t *taskfile.Task) bool { return t == a.task })
	if ended {
		set(a.task, "ended", today())
		*a.siblings = append(list, a.task)
		return
	}
	set(a.task, "ended", "")
	*a.siblings = slices.Insert(list, firstEnded(list), a.task)
}

// find is the way down to the Task with the id, outermost first, or nil.
func find(f *taskfile.File, id string) []at {
	var walk func(*[]*taskfile.Task) []at
	walk = func(list *[]*taskfile.Task) []at {
		for _, t := range *list {
			here := at{t, list}
			if t.ID == id {
				return []at{here}
			}
			if below := walk(&t.Subtasks); below != nil {
				return append([]at{here}, below...)
			}
		}
		return nil
	}
	return walk(&f.Tasks)
}

// parse reads the file a write is about to rewrite, refusing one with a line
// the rewrite would drop.
func parse(text string) (*taskfile.File, error) {
	f, problems := taskfile.Parse(text)
	for _, p := range problems {
		if p.Lost {
			return nil, Refused{fmt.Sprintf("TASKS.md line %d: %s, so a write would drop it; fix it by hand (tasks lint)", p.Line, p.Message)}
		}
	}
	return f, nil
}

func known(s taskfile.State) error {
	if !slices.Contains(taskfile.States, s) {
		return Invalid{fmt.Errorf("%q is not a State: want one of %v", s, taskfile.States)}
	}
	return nil
}

func reasonFits(reason string, s taskfile.State) error {
	if reason != "" && s != taskfile.Deferred && s != taskfile.Declined {
		return Invalid{fmt.Errorf("a Reason is held only while a Task is Deferred or Declined, not %s", title(s))}
	}
	return nil
}

// set gives the Task the attribute, replacing one it has, or takes it off
// when the value is empty.
func set(t *taskfile.Task, label, value string) {
	i := slices.IndexFunc(t.Attrs, func(a taskfile.Attr) bool { return a.Label == label })
	switch {
	case value == "" && i >= 0:
		t.Attrs = slices.Delete(t.Attrs, i, i+1)
	case value == "":
	case i >= 0:
		t.Attrs[i].Value = value
	default:
		t.Attrs = append(t.Attrs, taskfile.Attr{Label: label, Value: value})
	}
}

func firstEnded(list []*taskfile.Task) int {
	if i := slices.IndexFunc(list, func(t *taskfile.Task) bool { return t.State.Ended() }); i >= 0 {
		return i
	}
	return len(list)
}

func every(tasks []*taskfile.Task) map[string]struct{} {
	ids := map[string]struct{}{}
	var walk func([]*taskfile.Task)
	walk = func(ts []*taskfile.Task) {
		for _, t := range ts {
			ids[t.ID] = struct{}{}
			walk(t.Subtasks)
		}
	}
	walk(tasks)
	return ids
}

// fresh is an id no Task in the Repo has: four lowercase letters or digits.
func fresh(taken map[string]struct{}) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	for {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		for i := range b {
			b[i] = alphabet[int(b[i])%len(alphabet)]
		}
		if _, ok := taken[string(b)]; !ok {
			return string(b)
		}
	}
}

// today is the host's local date, which is what "today" means everywhere.
func today() string { return time.Now().Format(time.DateOnly) }

func title(s taskfile.State) string { return strings.ToUpper(string(s[:1])) + string(s[1:]) }
