// Package board is one read of every Repo's Tasks: discover the Repos, parse
// each tasks.md afresh, work out what a read works out (Blocked, a Task's
// parents), and keep what the Narrowing asks for. `tasks list`, `tasks repos`,
// `tasks lint` and GET /api/state all read through it, so a person and an
// Agent are answered from the same call.
package board

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/repos"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// Narrowing is what one read asks for. The zero Narrowing is every Task in
// every Repo. Naming several States or several Tags widens within the field:
// a Task in any one of them is in.
type Narrowing struct {
	Repo      string
	States    []taskfile.State
	Tags      []string
	Search    string
	Unblocked bool // leave Blocked Tasks out
	Sort      Sort // empty is file order
}

// Check refuses a Narrowing naming a State or a Sort there is not.
func (n Narrowing) Check() error {
	for _, s := range n.States {
		if !slices.Contains(taskfile.States, s) {
			return fmt.Errorf("%q is not a State: want one of %v", s, taskfile.States)
		}
	}
	if n.Sort != "" && !slices.Contains(Sorts, n.Sort) {
		return fmt.Errorf("%q is not a sort: want one of %v", n.Sort, Sorts)
	}
	return nil
}

// Sort is an order a read comes back in. Within a Repo it orders siblings and
// never flattens the tree, so a Subtask still sits under its parent; across
// Repos it is each Task's Rank.
type Sort string

const (
	SortFile     Sort = "file"
	SortTitle    Sort = "title"
	SortDeadline Sort = "deadline"
	SortCreated  Sort = "created"
	SortPriority Sort = "priority"
	SortEstimate Sort = "estimate"
)

// Sorts is every Sort there is, in the order a surface offers them. The flag
// help, the refusal of an unknown one and the sorts a read serves all come
// from it, so no surface keeps a copy that could offer one this would refuse.
var Sorts = []Sort{SortFile, SortTitle, SortDeadline, SortCreated, SortPriority, SortEstimate}

// ranked is the order each level's words sort in. A word not listed, and a
// Task without the attribute, sorts after every one that has it.
var ranked = map[Sort]map[string]string{
	SortPriority: {"high": "0", "med": "1", "medium": "1", "low": "2"},
	SortEstimate: {"s": "0", "small": "0", "m": "1", "medium": "1", "l": "2", "large": "2"},
}

// key is what the Sort compares a Task by: two keys in string order are the
// two Tasks in the Sort's order. File order is one key for every Task, so a
// stable sort leaves them where they were.
func (s Sort) key(title, created string, attr func(string) string) string {
	const missing = "~" // after every digit and letter a key holds
	switch s {
	case SortTitle:
		return strings.ToLower(title)
	case SortCreated:
		return cmp.Or(created, missing)
	case SortDeadline:
		return cmp.Or(attr("deadline"), missing)
	case SortPriority, SortEstimate:
		return cmp.Or(ranked[s][strings.ToLower(attr(string(s)))], missing)
	}
	return ""
}

// Task is one Task as a read gives it: flat, in file order, carrying where it
// sits in its tree and what the read worked out about it.
type Task struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	State       taskfile.State  `json:"state"`
	Tags        []string        `json:"tags"`
	Created     string          `json:"created"`
	Attrs       []taskfile.Attr `json:"attrs"`
	Description string          `json:"description"`
	Line        int             `json:"line"`
	Depth       int             `json:"depth"`
	Parent      string          `json:"parent"`
	// Parents is every title above this Task, outermost first.
	Parents []string `json:"parents"`
	Leaf    bool     `json:"leaf"`
	Blocked bool     `json:"blocked"`
	// Rank is this Task's place in the whole read's order, counted from 0
	// across every Repo: by the Sort, then Repo, then place in the file. A
	// surface drawing Tasks from several Repos together orders them by it.
	Rank int `json:"rank"`
}

func (t Task) attr(label string) string {
	for _, a := range t.Attrs {
		if a.Label == label {
			return a.Value
		}
	}
	return ""
}

// Repo is one Repo's read. Problems is the file's lint, by line: a Repo with
// any is flagged wherever it is drawn.
type Repo struct {
	Name     string             `json:"name"`
	Path     string             `json:"path"`
	Color    string             `json:"color"`
	Tasks    []Task             `json:"tasks"`
	Problems []taskfile.Problem `json:"problems"`
}

// Board is a whole read as `tasks list -json` prints it and GET /api/state
// answers it: every Repo read, the config errors met finding them, the sorts
// a read can be ordered by, and the host's local date, which is the day a
// Deadline is today or Overdue against.
type Board struct {
	Repos  []Repo   `json:"repos"`
	Errors []string `json:"errors"`
	Sorts  []Sort   `json:"sorts"`
	Today  string   `json:"today"`
}

// Of is the Board holding a read, with errs as its sentences, never null.
func Of(read []Repo, errs []error) Board {
	sentences := make([]string, len(errs))
	for i, err := range errs {
		sentences[i] = err.Error()
	}
	return Board{Repos: read, Errors: sentences, Sorts: Sorts, Today: Today()}
}

// Today is the host's local date, written the way a Deadline is.
func Today() string { return time.Now().Format(time.DateOnly) }

// NoRepo is a Narrowing naming a Repo that is not there.
type NoRepo string

func (n NoRepo) Error() string { return fmt.Sprintf("no Repo is named %s", string(n)) }

// Discover reads the config and scans its roots. Errors are config errors:
// a config that cannot be read, a root that is not there, a name under two
// roots. They never stop the Repos that were found from being read.
func Discover() ([]repos.Repo, []error) {
	path, err := repos.ConfigPath()
	if err != nil {
		return nil, []error{err}
	}
	roots, err := repos.Roots(path)
	if err != nil {
		return nil, []error{err}
	}
	return repos.Find(roots)
}

// Read reads every Repo found, narrowed. Every Repo is in the result, holding
// whatever of its Tasks the Narrowing kept, unless the Narrowing names one.
func Read(found []repos.Repo, n Narrowing) ([]Repo, error) {
	found, err := Only(found, n.Repo)
	if err != nil {
		return nil, err
	}
	out := make([]Repo, 0, len(found))
	for _, r := range found {
		read := readRepo(r, n.Sort)
		kept := read.Tasks[:0]
		for _, t := range read.Tasks {
			if n.keeps(t) {
				kept = append(kept, t)
			}
		}
		read.Tasks = kept
		out = append(out, read)
	}
	var all []*Task
	for i := range out {
		for j := range out[i].Tasks {
			all = append(all, &out[i].Tasks[j])
		}
	}
	slices.SortStableFunc(all, func(a, b *Task) int {
		return strings.Compare(n.Sort.key(a.Title, a.Created, a.attr), n.Sort.key(b.Title, b.Created, b.attr))
	})
	for rank, t := range all {
		t.Rank = rank
	}
	return out, nil
}

// Only is the Repo named, or every Repo when the name is empty.
func Only(found []repos.Repo, name string) ([]repos.Repo, error) {
	if name == "" {
		return found, nil
	}
	i := slices.IndexFunc(found, func(r repos.Repo) bool { return r.Name == name })
	if i < 0 {
		return nil, NoRepo(name)
	}
	return found[i : i+1], nil
}

// ReadRepo reads one Repo's file, whole and in file order. A file that cannot
// be read is a problem on line 0 rather than an error, so one broken Repo
// never hides the rest.
func ReadRepo(r repos.Repo) Repo { return readRepo(r, SortFile) }

func readRepo(r repos.Repo, sort Sort) Repo {
	out := Repo{Name: r.Name, Path: r.Path, Tasks: []Task{}, Problems: []taskfile.Problem{}}
	text, err := os.ReadFile(r.File())
	if err != nil {
		out.Problems = append(out.Problems, taskfile.Problem{Message: "cannot read tasks.md: " + unwrapped(err)})
		return out
	}
	f, problems := taskfile.Parse(string(text))
	out.Color = f.Color()
	out.Problems = append(out.Problems, problems...)

	ended := map[string]bool{}
	var index func([]*taskfile.Task)
	index = func(ts []*taskfile.Task) {
		for _, t := range ts {
			if t.ID != "" {
				ended[t.ID] = t.State.Ended()
			}
			index(t.Subtasks)
		}
	}
	index(f.Tasks)

	var flatten func(ts []*taskfile.Task, parent string, parents []string)
	flatten = func(ts []*taskfile.Task, parent string, parents []string) {
		ts = slices.Clone(ts)
		slices.SortStableFunc(ts, func(a, b *taskfile.Task) int {
			return strings.Compare(sort.key(a.Title, a.Created, a.Attr), sort.key(b.Title, b.Created, b.Attr))
		})
		for _, t := range ts {
			out.Tasks = append(out.Tasks, Task{
				ID:          t.ID,
				Title:       t.Title,
				State:       t.State,
				Tags:        orEmpty(t.Tags),
				Created:     t.Created,
				Attrs:       orEmpty(t.Attrs),
				Description: t.Description,
				Line:        t.Line,
				Depth:       len(parents),
				Parent:      parent,
				Parents:     parents,
				Leaf:        len(t.Subtasks) == 0,
				Blocked: slices.ContainsFunc(t.BlockedBy(), func(id string) bool {
					done, known := ended[id]
					return known && !done
				}),
			})
			flatten(t.Subtasks, t.ID, append(slices.Clip(parents), t.Title))
		}
	}
	flatten(f.Tasks, "", []string{})
	return out
}

func (n Narrowing) keeps(t Task) bool {
	if len(n.States) > 0 && !slices.Contains(n.States, t.State) {
		return false
	}
	if len(n.Tags) > 0 && !slices.ContainsFunc(n.Tags, func(tag string) bool { return slices.Contains(t.Tags, tag) }) {
		return false
	}
	if n.Unblocked && t.Blocked {
		return false
	}
	if n.Search != "" {
		needle := strings.ToLower(n.Search)
		hay := strings.ToLower(t.ID + "\n" + t.Title + "\n" + t.Description)
		if !strings.Contains(hay, needle) {
			return false
		}
	}
	return true
}

// orEmpty is s, or an empty slice where s is nil, so JSON says [] and not null.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func unwrapped(err error) string {
	var path *os.PathError
	if errors.As(err, &path) {
		return path.Err.Error()
	}
	return err.Error()
}
