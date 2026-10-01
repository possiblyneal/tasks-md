package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/history"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/repos"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/write"
)

// add is `tasks add <title> [flags]`: a new Task in the Repo the command is
// run in, or the one -repo names. It prints the new id.
func add(args []string, stdout, stderr io.Writer) int {
	fs := flags("add", stderr)
	var n write.New
	var tags, blockers repeated
	repo := fs.String("repo", "", "the Repo to add to, rather than the one the command is run in")
	fs.StringVar(&n.State, "state", "", "the State it starts in, inbox when not given: "+strings.Join(stateWords(), ", "))
	fs.Var(&tags, "tag", "a Tag it carries, repeatable")
	fs.StringVar(&n.Description, "description", "", "what it is, at length")
	fs.StringVar(&n.Why, "why", "", "why it is worth doing")
	fs.StringVar(&n.Acceptance, "acceptance", "", "how anybody can tell it is done")
	fs.StringVar(&n.Deadline, "deadline", "", "the date it is due, YYYY-MM-DD")
	fs.StringVar(&n.Priority, "priority", "", "low, med or high")
	fs.StringVar(&n.Impact, "impact", "", "low, med or high")
	fs.StringVar(&n.Estimate, "estimate", "", "small, medium or large")
	fs.StringVar(&n.Color, "color", "", "the color its card is drawn in")
	fs.Var(&blockers, "blocked-by", "the id of a Task in the same Repo it waits on, repeatable")
	fs.StringVar(&n.Reason, "reason", "", "why, when it starts Deferred or Declined")
	words, err := parse(fs, args)
	if err != nil {
		return 2
	}
	n.Title, n.Tags, n.BlockedBy = strings.Join(words, " "), tags, blockers
	dir, err := where(*repo)
	if err != nil {
		return failed("add", err, stderr)
	}
	id, err := write.Add(dir, actor(), n)
	if err != nil {
		return failed("add", err, stderr)
	}
	fmt.Fprintln(stdout, id)
	warn("add", dir, stderr)
	return 0
}

// move is `tasks move <id> <state> [-reason r]`.
func move(args []string, _, stderr io.Writer) int {
	fs := flags("move", stderr)
	repo := fs.String("repo", "", "the Repo the Task is in, rather than the one the command is run in")
	reason := fs.String("reason", "", "why, when the Task is moved to Deferred or Declined")
	words, err := parse(fs, args)
	if err != nil {
		return 2
	}
	if len(words) != 2 {
		fmt.Fprintf(stderr, "tasks move: want an id and a State, got %q\n", words)
		return 2
	}
	dir, err := where(*repo)
	if err != nil {
		return failed("move", err, stderr)
	}
	if err := write.Move(dir, actor(), words[0], taskfile.State(words[1]), *reason); err != nil {
		return failed("move", err, stderr)
	}
	warn("move", dir, stderr)
	return 0
}

// parse reads flags wherever they fall among the words, so `tasks add Buy
// milk -tag errand` and `tasks add -tag errand Buy milk` are one command.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var words []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return words, nil
		}
		words = append(words, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// failed prints a write's error and gives the exit status its kind carries:
// 3 for a refusal, 2 for usage, 1 for anything else.
func failed(verb string, err error, stderr io.Writer) int {
	fmt.Fprintf(stderr, "tasks %s: %v\n", verb, err)
	var invalid write.Invalid
	switch {
	case write.IsRefused(err):
		return 3
	case errors.As(err, &invalid):
		return 2
	}
	return 1
}

// warn says what a write that went through left the Repo flagged with, such
// as a push that did not go through. It is a warning: the write is done.
func warn(verb, dir string, stderr io.Writer) {
	for _, flag := range history.Flags(dir) {
		fmt.Fprintf(stderr, "tasks %s: warning: %s\n", verb, flag)
	}
}

// actor is who a write is made as: TASKS_ACTOR, else the config's name, else
// the login name.
func actor() string {
	if a := os.Getenv("TASKS_ACTOR"); a != "" {
		return a
	}
	if path, err := repos.ConfigPath(); err == nil {
		if c, err := repos.Load(path); err == nil && c.Name != "" {
			return c.Name
		}
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "unknown"
}

// where is the folder a write acts on: the Repo named, or else the nearest
// tasks.md at or above the working directory. In a linked git worktree that
// is looked for from the main checkout, since the worktree's own copy of the
// folder is not where the Repo's Tasks live.
func where(name string) (string, error) {
	if name != "" {
		r, err := board.Named(name)
		return r.Path, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	start := history.MainCheckout(cwd)
	for dir := start; ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(filepath.Join(dir, "tasks.md")); err == nil && info.Mode().IsRegular() {
			return filepath.EvalSymlinks(dir)
		}
		if dir == filepath.Dir(dir) {
			return "", fmt.Errorf("no tasks.md at or above %s; name a Repo with -repo", start)
		}
	}
}
