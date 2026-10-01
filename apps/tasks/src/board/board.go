// Package board is one read of every Repo's Tasks: discover the Repos, parse
// each tasks.md afresh, work out what a read works out (Blocked, a Task's
// parents), and keep what the Narrowing asks for. `tasks list`, `tasks repos`,
// `tasks lint` and GET /api/state all read through it, so a person and an
// Agent are answered from the same call.
package board

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

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
}

// Check refuses a Narrowing naming a State there is not.
func (n Narrowing) Check() error {
	for _, s := range n.States {
		if !slices.Contains(taskfile.States, s) {
			return fmt.Errorf("%q is not a State: want one of %v", s, taskfile.States)
		}
	}
	return nil
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
// answers it: every Repo read, and the config errors met finding them.
type Board struct {
	Repos  []Repo   `json:"repos"`
	Errors []string `json:"errors"`
}

// Errors is errs as the sentences a Board carries, never null.
func Errors(errs []error) []string {
	out := make([]string, len(errs))
	for i, err := range errs {
		out[i] = err.Error()
	}
	return out
}

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
		read := ReadRepo(r)
		kept := read.Tasks[:0]
		for _, t := range read.Tasks {
			if n.keeps(t) {
				kept = append(kept, t)
			}
		}
		read.Tasks = kept
		out = append(out, read)
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

// ReadRepo reads one Repo's file, whole. A file that cannot be read is a
// problem on line 0 rather than an error, so one broken Repo never hides the
// rest.
func ReadRepo(r repos.Repo) Repo {
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
