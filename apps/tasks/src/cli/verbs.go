package cli

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// flags is a verb's flag set, reporting to stderr and leaving the exit status
// to the verb.
func flags(verb string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("tasks "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// repeated collects a flag that may be given more than once.
type repeated []string

func (v *repeated) String() string { return strings.Join(*v, ",") }

func (v *repeated) Set(s string) error {
	*v = append(*v, s)
	return nil
}

// configErrors prints what was wrong with the config and says whether
// anything was.
func configErrors(errs []error, stderr io.Writer) bool {
	for _, err := range errs {
		fmt.Fprintf(stderr, "tasks: %v\n", err)
	}
	return len(errs) > 0
}

// list is `tasks list`: every Repo's Tasks, grouped by Repo, narrowed by the
// flags. A config error is printed and makes the exit 1, after whatever Repos
// were found are listed.
func list(args []string, stdout, stderr io.Writer) int {
	fs := flags("list", stderr)
	var n board.Narrowing
	var states, tags repeated
	fs.StringVar(&n.Repo, "repo", "", "only the Repo with this name")
	fs.Var(&states, "state", "only Tasks in this State, repeatable: "+strings.Join(stateWords(), ", "))
	fs.Var(&tags, "tag", "only Tasks carrying this Tag, repeatable")
	fs.StringVar(&n.Search, "search", "", "only Tasks whose id, title or description holds this text")
	fs.BoolVar(&n.Unblocked, "unblocked", false, "leave Blocked Tasks out")
	sort := fs.String("sort", string(board.SortFile), "order siblings by one of: "+strings.Join(sortWords(), ", "))
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "tasks list: takes no arguments, got %q\n", fs.Args())
		return 2
	}
	for _, s := range states {
		n.States = append(n.States, taskfile.State(s))
	}
	n.Tags = tags
	n.Sort = board.Sort(*sort)
	if err := n.Check(); err != nil {
		fmt.Fprintf(stderr, "tasks list: %v\n", err)
		return 2
	}

	found, errs := board.Discover()
	if only, err := board.Only(found, n.Repo); err == nil {
		pass(only...)
	}
	read, err := board.Read(found, n)
	if err != nil {
		configErrors(errs, stderr)
		fmt.Fprintf(stderr, "tasks list: %v\n", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		// Read has already refused a Repo there is not, so Weekly cannot.
		weeks, _ := board.Weekly(found, n, now())
		if err := enc.Encode(board.Of(read, weeks, errs)); err != nil {
			fmt.Fprintf(stderr, "tasks list: %v\n", err)
			return 1
		}
	} else {
		for _, r := range read {
			fmt.Fprintf(stdout, "%s %s\n", r.Name, r.Path)
			for _, t := range r.Tasks {
				fmt.Fprintf(stdout, "%s%s %s %s", strings.Repeat("  ", t.Depth+1), orDash(t.ID), t.State, t.Title)
				for _, tag := range t.Tags {
					fmt.Fprintf(stdout, " #%s", tag)
				}
				if t.Blocked {
					fmt.Fprint(stdout, " blocked")
				}
				fmt.Fprintln(stdout)
			}
			for _, p := range r.Problems {
				fmt.Fprintf(stdout, "  ! line %d: %s\n", p.Line, p.Message)
			}
		}
	}
	if configErrors(errs, stderr) {
		return 1
	}
	return 0
}

func stateWords() []string {
	words := make([]string, len(taskfile.States))
	for i, s := range taskfile.States {
		words[i] = string(s)
	}
	return words
}

func sortWords() []string {
	words := make([]string, len(board.Sorts))
	for i, s := range board.Sorts {
		words[i] = string(s)
	}
	return words
}

// orDash stands in for an id a hand-typed Task does not have yet.
func orDash(id string) string {
	if id == "" {
		return "-"
	}
	return id
}

// listRepos is `tasks repos`: every Repo the tracker sees, where it lives, and
// how many of its Tasks are in each State, and what its tasks history is
// flagged with, - for nothing. It is read-only: a Repo is made,
// renamed or removed as a folder.
func listRepos(args []string, stdout, stderr io.Writer) int {
	fs := flags("repos", stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	found, errs := board.Discover()
	pass(found...)
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tPATH\t%s\tFLAGS\n", strings.ToUpper(strings.Join(stateWords(), "\t")))
	for _, r := range found {
		read := board.ReadRepo(r)
		counts := map[taskfile.State]int{}
		for _, t := range read.Tasks {
			counts[t.State]++
		}
		fmt.Fprintf(w, "%s\t%s", r.Name, r.Path)
		for _, s := range taskfile.States {
			fmt.Fprintf(w, "\t%d", counts[s])
		}
		fmt.Fprintf(w, "\t%s\n", cmp.Or(strings.Join(read.Flags, ", "), "-"))
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "tasks repos: %v\n", err)
		return 1
	}
	if configErrors(errs, stderr) {
		return 1
	}
	return 0
}

// lint is `tasks lint [file...]`: every problem in the named files, or in
// every Repo's TASKS.md when none is named, one `file:line: message` a line.
// It exits 1 when there is any.
func lint(args []string, stdout, stderr io.Writer) int {
	fs := flags("lint", stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	var errs []error
	if len(files) == 0 {
		found, discovered := board.Discover()
		errs = discovered
		pass(found...)
		for _, r := range found {
			files = append(files, r.File())
		}
	}
	bad := false
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		_, problems := taskfile.Parse(string(text))
		for _, p := range problems {
			fmt.Fprintf(stdout, "%s:%d: %s\n", file, p.Line, p.Message)
			bad = true
		}
	}
	if configErrors(errs, stderr) || bad {
		return 1
	}
	return 0
}
