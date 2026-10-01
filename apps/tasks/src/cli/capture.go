package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/ai"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/write"
)

// capture is `tasks capture <words> [-repo r] [-dry]`: a dump read by the
// Broker and written as one Task, the one verb that writes what the Broker
// said without a form, because the words are the caller's own. It goes to the
// Repo the command is run in or the one -repo names, never to the Repo the
// Broker guessed. -dry prints what was read and writes nothing.
func capture(args []string, stdout, stderr io.Writer) int {
	fs := flags("capture", stderr)
	repo := fs.String("repo", "", "the Repo to add to, rather than the one the command is run in")
	dry := fs.Bool("dry", false, "print what the Broker read out of it and write nothing")
	words, err := parse(fs, args)
	if err != nil {
		return 2
	}
	text := strings.TrimSpace(strings.Join(words, " "))
	if text == "" {
		fmt.Fprintln(stderr, "tasks capture: say what the Task is")
		return 2
	}
	dir, err := where(*repo)
	if err != nil {
		return failed("capture", err, stderr)
	}
	found, _ := board.Discover()
	read, err := board.Read(found, board.Narrowing{})
	if err != nil {
		return failed("capture", err, stderr)
	}
	dump := write.DumpOf(text, read, now())
	ctx, cancel := context.WithTimeout(context.Background(), ai.Patience)
	defer cancel()
	said, err := ai.New().Read(ctx, dump)
	if err != nil {
		return failed("capture", err, stderr)
	}
	if *dry {
		printCapture(stdout, said)
		return 0
	}
	n, err := write.FromCapture(said, dump.Tags)
	if err != nil {
		return failed("capture", err, stderr)
	}
	id, err := write.Add(dir, actor(), n)
	if err != nil {
		return failed("capture", err, stderr)
	}
	fmt.Fprintln(stdout, id)
	warn("capture", dir, stderr)
	return 0
}

// printCapture is -dry: what the Broker made of the dump, unparsed, under the
// names the attributes carry everywhere else.
func printCapture(stdout io.Writer, said ai.Capture) {
	for _, line := range [][2]string{
		{"title", said.Title},
		{"description", said.Description},
		{"why", said.Why},
		{"deadline", said.Deadline},
		{"estimate", said.Estimate},
		{"priority", said.Priority},
		{"impact", said.Impact},
		{"repo", said.Repo},
		{"tags", strings.Join(said.Tags, ", ")},
	} {
		if strings.TrimSpace(line[1]) != "" {
			fmt.Fprintf(stdout, "%-11s %s\n", line[0], line[1])
		}
	}
}
