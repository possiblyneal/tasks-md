package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// stats is `tasks stats`: the board's lane counts, a card being a leaf Task,
// and the weekly metrics, narrowed by -repo and -tag the way the board's chips
// narrow it.
func stats(args []string, stdout, stderr io.Writer) int {
	fs := flags("stats", stderr)
	var n board.Narrowing
	var tags repeated
	fs.StringVar(&n.Repo, "repo", "", "only the Repo with this name")
	fs.Var(&tags, "tag", "only Tasks carrying this Tag, repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "tasks stats: takes no arguments, got %q\n", fs.Args())
		return 2
	}
	n.Tags = tags

	found, errs := board.Discover()
	if only, err := board.Only(found, n.Repo); err == nil {
		pass(only...)
	}
	read, err := board.Read(found, n)
	if err != nil {
		configErrors(errs, stderr)
		fmt.Fprintf(stderr, "tasks stats: %v\n", err)
		return 1
	}
	weeks, err := board.Weekly(found, n, now())
	if err != nil {
		fmt.Fprintf(stderr, "tasks stats: %v\n", err)
		return 1
	}

	cards := map[taskfile.State]int{}
	for _, r := range read {
		for _, t := range r.Tasks {
			if t.Leaf {
				cards[t.State]++
			}
		}
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "LANE\tCARDS")
	for _, s := range taskfile.States {
		fmt.Fprintf(w, "%s\t%d\n", s, cards[s])
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "WEEK\tADDED\tDONE\tDECLINED")
	for _, week := range weeks {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", week.Start, week.Added, week.Done, week.Declined)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "tasks stats: %v\n", err)
		return 1
	}
	if configErrors(errs, stderr) {
		return 1
	}
	return 0
}
