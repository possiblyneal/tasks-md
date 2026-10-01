package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/repos"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/write"
)

// now is the host's clock, which the pass reads "today" from. Tests set it.
var now = time.Now

// pass runs the pass over the Repos a verb is about to read or write.
func pass(found ...repos.Repo) {
	dirs := make([]string, len(found))
	for i, r := range found {
		dirs[i] = r.Path
	}
	write.Pass(now(), dirs...)
}

// repeat is `tasks repeat <id> [rule...] [-off]`: put a Series on the Task,
// or take it off with -off, and print the rule and its next dates. With no
// rule and no -off it prints them and writes nothing.
func repeat(args []string, stdout, stderr io.Writer) int {
	fs := flags("repeat", stderr)
	repo := fs.String("repo", "", "the Repo the Task is in, rather than the one the command is run in")
	off := fs.Bool("off", false, "take the Series off, leaving an ordinary Task")
	words, err := parse(fs, args)
	if err != nil {
		return 2
	}
	if len(words) == 0 || (*off && len(words) > 1) {
		fmt.Fprintf(stderr, "tasks repeat: want an id and a rule such as %q, or an id and -off\n", "every week on mon")
		return 2
	}
	id, rule := words[0], strings.Join(words[1:], " ")
	dir, err := where(*repo)
	if err != nil {
		return failed("repeat", err, stderr)
	}
	if rule != "" || *off {
		if err := write.Repeat(dir, actor(), id, rule); err != nil {
			return failed("repeat", err, stderr)
		}
		warn("repeat", dir, stderr)
	}
	s, err := write.SeriesOf(dir, id, now())
	if err != nil {
		return failed("repeat", err, stderr)
	}
	if s.Rule == "" {
		fmt.Fprintf(stdout, "%s does not repeat\n", id)
		return 0
	}
	fmt.Fprint(stdout, s)
	return 0
}
