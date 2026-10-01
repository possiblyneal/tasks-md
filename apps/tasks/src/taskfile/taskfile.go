// Package taskfile is the tasks.md format: Parse reads a Repo's file into its
// Task tree plus every problem found, by line, and Write gives the tree back
// as canonical text. The format is settled in issues #97 and #105:
//
//	# Tasks
//	color: green
//	<!-- a comment for Agents -->
//
//	- [ ] Pack the kitchen | doing #packing #kitchen
//	  - id: m3qa
//	  - created: 2026-09-20
//	  - deadline: 2026-10-10
//
//	  Start with the glassware.
//
//	  - [x] Buy boxes and tape | done
//	    - id: m3qb
//	    - created: 2026-09-20
//
// Everything above the first Task is the preamble and is kept verbatim. Lint
// is Parse's problems; nothing else in the tracker judges a file.
package taskfile

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// State is where a Task stands, as the word written on its title line.
type State string

const (
	Inbox    State = "inbox"
	Backlog  State = "backlog"
	Doing    State = "doing"
	Deferred State = "deferred"
	Done     State = "done"
	Declined State = "declined"
)

// States is all six, in the order the board draws its lanes.
var States = []State{Inbox, Backlog, Doing, Deferred, Done, Declined}

// Ended says whether the State ends a Task.
func (s State) Ended() bool { return s == Done || s == Declined }

// Attr is one `- label: value` line under a title, other than id and created.
// Known and unknown labels alike are kept in the order they were read.
type Attr struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Task is one title line and everything indented under it.
//
// State is the State that counts: the checkbox over the word, and for a
// parent the State worked out from its Subtasks. Line is the title's line
// number, counted from 1.
type Task struct {
	Line        int
	Title       string
	State       State
	Tags        []string
	ID          string
	Created     string
	Attrs       []Attr
	Description string
	Subtasks    []*Task
}

// Attr is the value of the first attribute carrying the label, or "".
func (t *Task) Attr(label string) string {
	for _, a := range t.Attrs {
		if a.Label == label {
			return a.Value
		}
	}
	return ""
}

// BlockedBy is the IDs on the Task's `blocked by` line.
func (t *Task) BlockedBy() []string {
	return strings.FieldsFunc(t.Attr("blocked by"), func(r rune) bool { return r == ',' || r == ' ' })
}

// File is a whole tasks.md.
type File struct {
	Preamble string
	Tasks    []*Task
}

// Color is the Repo's Color, from the preamble's `color:` line.
func (f *File) Color() string {
	for line := range strings.Lines(f.Preamble) {
		if value, ok := strings.CutPrefix(line, "color:"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Problem is one thing wrong with a file, at the line it is on. Lost says the
// line could not be placed in any Task, so Write would drop it.
type Problem struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
	Lost    bool   `json:"-"`
}

var (
	titleLine = regexp.MustCompile(`^( *)- \[(.)\] (.*)$`)
	attrLine  = regexp.MustCompile(`^- ([a-z][a-z ]*):(?: (.*))?$`)
	idForm    = regexp.MustCompile(`^[a-z0-9]{4}$`)
)

// maxDepth is how deep a Subtask may sit: a top-level Task is depth 0, and a
// Subtask nests to five levels below it.
const maxDepth = 5

// open is a Task being read, with what Parse needs to judge it afterwards.
type open struct {
	task    *Task
	indent  int
	written State
	// labels are the attribute labels in the order read, id and created
	// included, each beside its line.
	labels    []string
	lines     []int
	described bool // a blank line has closed the attributes
	desc      []string
}

// Parse reads a tasks.md. It always returns a File, holding whatever could be
// read, and the problems sorted by line.
func Parse(text string) (*File, []Problem) {
	f := &File{}
	var problems []Problem
	problem := func(line int, format string, args ...any) {
		problems = append(problems, Problem{Line: line, Message: fmt.Sprintf(format, args...)})
	}
	lost := func(line int, message string) {
		problems = append(problems, Problem{Line: line, Message: message, Lost: true})
	}

	var stack []*open // the Task each depth is currently under
	var all []*open
	preamble := true
	var head strings.Builder

	n := 0
	for raw := range strings.Lines(text) {
		n++
		line := strings.TrimRight(raw, "\r\n")
		if conflict(line) {
			problem(n, "conflict marker")
			continue
		}
		if m := titleLine.FindStringSubmatch(line); m != nil && (!preamble || m[1] == "") {
			preamble = false
			indent := len(m[1])
			depth := indent / 2
			if indent%2 != 0 || depth > len(stack) {
				lost(n, "a Subtask indented past its parent")
				continue
			}
			if depth > maxDepth {
				problem(n, "a Subtask nested deeper than %d levels", maxDepth)
			}
			o := &open{task: &Task{Line: n}, indent: indent}
			o.written = title(o.task, m[2], m[3], func(format string, args ...any) { problem(n, format, args...) })
			stack = stack[:depth]
			if depth == 0 {
				f.Tasks = append(f.Tasks, o.task)
			} else {
				parent := stack[depth-1].task
				parent.Subtasks = append(parent.Subtasks, o.task)
			}
			stack = append(stack, o)
			all = append(all, o)
			continue
		}
		if preamble {
			head.WriteString(line + "\n")
			continue
		}
		if strings.TrimSpace(line) == "" {
			if len(stack) > 0 {
				o := stack[len(stack)-1]
				o.described = true
				if len(o.desc) > 0 {
					o.desc = append(o.desc, "")
				}
			}
			continue
		}
		// Any other line belongs to the deepest open Task it is indented
		// under, and only while that Task has no Subtasks yet.
		indent := len(line) - len(strings.TrimLeft(line, " "))
		var owner *open
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].indent+2 <= indent {
				owner = stack[i]
				stack = stack[:i+1]
				break
			}
		}
		if owner == nil || len(owner.task.Subtasks) > 0 {
			lost(n, "cannot read this line as part of a Task")
			continue
		}
		content := line[owner.indent+2:]
		if m := attrLine.FindStringSubmatch(content); m != nil && !owner.described {
			attr(owner, m[1], m[2], n)
			continue
		}
		owner.described = true
		owner.desc = append(owner.desc, content)
	}

	f.Preamble = head.String()
	for _, o := range all {
		o.task.Description = strings.TrimRight(strings.Join(o.desc, "\n"), "\n")
		fixed(o, problem)
	}
	f.Settle()
	for _, o := range all {
		if len(o.task.Subtasks) > 0 && o.written != o.task.State {
			problem(o.task.Line, "the line says %s, but its Subtasks make it %s", o.written, o.task.State)
		}
	}
	references(all, problem)
	slices.SortStableFunc(problems, func(a, b Problem) int { return cmp.Compare(a.Line, b.Line) })
	return f, problems
}

func conflict(line string) bool {
	for _, marker := range []string{"<<<<<<<", "=======", ">>>>>>>", "|||||||"} {
		if strings.HasPrefix(line, marker) {
			return true
		}
	}
	return false
}

// title reads a title line's box and text into t, and returns the State the
// line says, which is the checkbox over the word.
func title(t *Task, box, text string, problem func(string, ...any)) State {
	word := ""
	if i := strings.LastIndex(text, " | "); i >= 0 {
		tail := strings.Fields(text[i+3:])
		text = text[:i]
		if len(tail) > 0 {
			word = tail[0]
			for _, tag := range tail[1:] {
				name, ok := strings.CutPrefix(tag, "#")
				if !ok || name == "" {
					problem("%q after the State is not a #tag", tag)
					continue
				}
				t.Tags = append(t.Tags, name)
			}
		}
	}
	t.Title = strings.TrimSpace(text)

	said := State(word)
	if word == "" {
		said = Inbox
	} else if !slices.Contains(States, said) {
		problem("%q is not a State", word)
		said = Inbox
	}
	switch box {
	case "x", "X":
		said = Done
	case "-":
		said = Declined
	case " ":
		if said.Ended() {
			said = Backlog
		}
	default:
		problem("[%s] is not a checkbox this format writes", box)
	}
	t.State = said
	return said
}

func attr(o *open, label, value string, line int) {
	o.labels = append(o.labels, label)
	o.lines = append(o.lines, line)
	value = strings.TrimSpace(value)
	switch label {
	case "id":
		o.task.ID = value
	case "created":
		o.task.Created = value
	default:
		o.task.Attrs = append(o.task.Attrs, Attr{label, value})
	}
}

// fixed checks the two lines every title carries, in order: id, then created.
func fixed(o *open, problem func(int, string, ...any)) {
	for want, label := range []string{"id", "created"} {
		at := slices.Index(o.labels, label)
		switch {
		case at < 0:
			problem(o.task.Line, "%q has no %s line", o.task.Title, label)
		case at != want && label == "id":
			problem(o.lines[at], "id is not the first line under the title")
		case at != want && slices.Contains(o.labels, "id"):
			problem(o.lines[at], "created is not the line after id")
		}
	}
	if at := slices.Index(o.labels, "id"); at >= 0 && !idForm.MatchString(o.task.ID) {
		problem(o.lines[at], "id %q is not four lowercase letters or digits", o.task.ID)
	}
}

// Settle works out every parent's State from its Subtasks again, after a
// write has changed one of them.
func (f *File) Settle() {
	for _, t := range f.Tasks {
		settle(t)
	}
}

// settle works out every parent's State from its Subtasks: while any is open,
// the first of Doing, Backlog, Inbox and Deferred among them; once all have
// ended, Declined if every one was, and Done otherwise.
func settle(t *Task) State {
	if len(t.Subtasks) == 0 {
		return t.State
	}
	var states []State
	for _, s := range t.Subtasks {
		states = append(states, settle(s))
	}
	t.State = Done
	if !slices.ContainsFunc(states, func(s State) bool { return s != Declined }) {
		t.State = Declined
	}
	for _, s := range []State{Doing, Backlog, Inbox, Deferred} {
		if slices.Contains(states, s) {
			t.State = s
			break
		}
	}
	return t.State
}

// references checks what one Task says about another: IDs are unique in the
// file, and a blocker names a Task the file holds.
func references(all []*open, problem func(int, string, ...any)) {
	seen := map[string]int{}
	for _, o := range all {
		if at := slices.Index(o.labels, "id"); at >= 0 && o.task.ID != "" {
			if first, ok := seen[o.task.ID]; ok {
				problem(o.lines[at], "id %s is also on line %d", o.task.ID, first)
			} else {
				seen[o.task.ID] = o.lines[at]
			}
		}
	}
	for _, o := range all {
		at := slices.Index(o.labels, "blocked by")
		for _, id := range o.task.BlockedBy() {
			if _, ok := seen[id]; !ok {
				problem(o.lines[at], "blocked by %s, which is no Task in this file", id)
			}
		}
	}
}

// Write gives the File back as canonical text: one shape for every Task,
// whatever spacing or words a hand edit left.
func Write(f *File) string {
	var b strings.Builder
	b.WriteString(f.Preamble)
	for _, t := range f.Tasks {
		write(&b, t, "")
	}
	return b.String()
}

func write(b *strings.Builder, t *Task, indent string) {
	box := " "
	switch t.State {
	case Done:
		box = "x"
	case Declined:
		box = "-"
	}
	fmt.Fprintf(b, "%s- [%s] %s | %s", indent, box, t.Title, t.State)
	for _, tag := range t.Tags {
		b.WriteString(" #" + tag)
	}
	b.WriteString("\n")

	under := indent + "  "
	line := func(label, value string) {
		if value == "" {
			fmt.Fprintf(b, "%s- %s:\n", under, label)
			return
		}
		fmt.Fprintf(b, "%s- %s: %s\n", under, label, value)
	}
	if t.ID != "" {
		line("id", t.ID)
	}
	if t.Created != "" {
		line("created", t.Created)
	}
	for _, a := range t.Attrs {
		line(a.Label, a.Value)
	}
	if t.Description != "" {
		b.WriteString("\n")
		for desc := range strings.Lines(t.Description + "\n") {
			if strings.TrimSpace(desc) == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString(under + desc)
		}
		if len(t.Subtasks) > 0 {
			b.WriteString("\n")
		}
	}
	for _, s := range t.Subtasks {
		write(b, s, under)
	}
}
