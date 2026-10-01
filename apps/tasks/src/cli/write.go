package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

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
	fs.StringVar(&n.Until, "until", "", "the date it is deferred until, YYYY-MM-DD, when it starts Deferred")
	fs.Var((*repeated)(&n.Attach), "attach", "a path or URL it points at, repeatable")
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

// move is `tasks move <id> <state> [-reason r] [-until date]`.
func move(args []string, _, stderr io.Writer) int {
	fs := flags("move", stderr)
	repo := fs.String("repo", "", "the Repo the Task is in, rather than the one the command is run in")
	reason := fs.String("reason", "", "why, when the Task is moved to Deferred or Declined")
	until := fs.String("until", "", "the date it is deferred until, YYYY-MM-DD, when it is moved to Deferred")
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
	if err := write.Move(dir, actor(), words[0], taskfile.State(words[1]), *reason, *until); err != nil {
		return failed("move", err, stderr)
	}
	warn("move", dir, stderr)
	return 0
}

// edit is `tasks edit <id> [flags]`: each flag given sets that attribute, an
// empty one takes it off, and one not given is left alone. -tag and
// -blocked-by replace the whole set. -version is the tree's version as
// `tasks list -json` gave it, so an edit made from what was read is refused
// if the tree changed since.
func edit(args []string, _, stderr io.Writer) int {
	fs := flags("edit", stderr)
	var e write.Edit
	var tags, blockers repeated
	repo := fs.String("repo", "", "the Repo the Task is in, rather than the one the command is run in")
	fs.StringVar(&e.Version, "version", "", "the tree's version as it was read, from tasks list -json")
	fields := []struct {
		name, usage string
		to          **string
	}{
		{"title", "its title", &e.Title},
		{"description", "what it is, at length", &e.Description},
		{"why", "why it is worth doing", &e.Why},
		{"acceptance", "how anybody can tell it is done", &e.Acceptance},
		{"deadline", "the date it is due, YYYY-MM-DD", &e.Deadline},
		{"priority", "low, med or high", &e.Priority},
		{"impact", "low, med or high", &e.Impact},
		{"estimate", "small, medium or large", &e.Estimate},
		{"color", "the color its card is drawn in", &e.Color},
		{"reason", "why, while it is Deferred or Declined", &e.Reason},
		{"until", "the date it is deferred until, YYYY-MM-DD, while it is Deferred", &e.Until},
	}
	values := make([]*string, len(fields))
	for i, f := range fields {
		values[i] = fs.String(f.name, "", f.usage)
	}
	fs.Var(&tags, "tag", "a Tag it carries, repeatable; replaces every Tag, and -tag '' takes them all off")
	fs.Var(&blockers, "blocked-by", "the id of a Task it waits on, repeatable; replaces every one")
	fs.Var((*repeated)(&e.Attach), "attach", "a path or URL to point it at, repeatable")
	fs.Var((*repeated)(&e.Detach), "detach", "a path or URL to stop pointing it at, repeatable")
	words, err := parse(fs, args)
	if err != nil {
		return 2
	}
	if len(words) != 1 {
		fmt.Fprintf(stderr, "tasks edit: want an id, got %q\n", words)
		return 2
	}
	fs.Visit(func(given *flag.Flag) {
		switch given.Name {
		case "tag":
			e.Tags = (*[]string)(&tags)
		case "blocked-by":
			e.BlockedBy = (*[]string)(&blockers)
		}
		for i, f := range fields {
			if f.name == given.Name {
				*f.to = values[i]
			}
		}
	})
	dir, err := where(*repo)
	if err != nil {
		return failed("edit", err, stderr)
	}
	if err := write.EditTask(dir, actor(), words[0], e); err != nil {
		return failed("edit", err, stderr)
	}
	warn("edit", dir, stderr)
	return 0
}

// remove is `tasks delete <id>`: the Task's block, Subtasks and all, goes from
// the file, and the tasks history keeps it.
func remove(args []string, _, stderr io.Writer) int {
	fs := flags("delete", stderr)
	repo := fs.String("repo", "", "the Repo the Task is in, rather than the one the command is run in")
	version := fs.String("version", "", "the tree's version as it was read, from tasks list -json")
	words, err := parse(fs, args)
	if err != nil {
		return 2
	}
	if len(words) != 1 {
		fmt.Fprintf(stderr, "tasks delete: want an id, got %q\n", words)
		return 2
	}
	dir, err := where(*repo)
	if err != nil {
		return failed("delete", err, stderr)
	}
	if err := write.Delete(dir, actor(), words[0], *version); err != nil {
		return failed("delete", err, stderr)
	}
	warn("delete", dir, stderr)
	return 0
}

// tags is `tasks tags`, every Tag with how many Tasks carry it across every
// Repo, and `tasks tags rename <from> <to>`, which rewrites every carrier as
// one commit in each Repo holding one.
func tags(args []string, stdout, stderr io.Writer) int {
	fs := flags("tags", stderr)
	words, err := parse(fs, args)
	if err != nil {
		return 2
	}
	found, errs := board.Discover()
	pass(found...)
	switch {
	case len(words) == 0:
		read, _ := board.Read(found, board.Narrowing{})
		counts := map[string]int{}
		for _, r := range read {
			for _, t := range r.Tasks {
				for _, tag := range t.Tags {
					counts[tag]++
				}
			}
		}
		w := tabwriter.NewWriter(stdout, 0, 0, 1, ' ', 0)
		for _, tag := range slices.Sorted(maps.Keys(counts)) {
			fmt.Fprintf(w, "%s\t%d\n", tag, counts[tag])
		}
		_ = w.Flush()
	case len(words) == 3 && words[0] == "rename":
		dirs := make([]string, len(found))
		for i, r := range found {
			dirs[i] = r.Path
		}
		written, err := write.RenameTag(dirs, actor(), words[1], words[2], nil)
		if err != nil {
			return failed("tags", err, stderr)
		}
		for _, dir := range written {
			warn("tags", dir, stderr)
		}
	default:
		fmt.Fprintf(stderr, "tasks tags: want nothing, or rename <from> <to>, got %q\n", words)
		return 2
	}
	if configErrors(errs, stderr) {
		return 1
	}
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

// where is the folder a write acts on, with the pass run over it first.
func where(name string) (string, error) {
	dir, err := nearest(name)
	if err == nil {
		write.Pass(now(), dir)
	}
	return dir, err
}

// nearest is the Repo named, or else the nearest TASKS.md at or above the
// working directory. In a linked git worktree that is looked for from the
// main checkout, since the worktree's own copy of the folder is not where the
// Repo's Tasks live.
func nearest(name string) (string, error) {
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
		if info, err := os.Stat(filepath.Join(dir, "TASKS.md")); err == nil && info.Mode().IsRegular() {
			return filepath.EvalSymlinks(dir)
		}
		if dir == filepath.Dir(dir) {
			return "", fmt.Errorf("no TASKS.md at or above %s; name a Repo with -repo", start)
		}
	}
}
