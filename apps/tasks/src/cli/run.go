package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/api"
)

// Run is the whole program behind main, taking its streams as arguments so the
// modes are testable without a process. It returns the exit status.
func Run(args []string, stdout, stderr io.Writer) int {
	switch ModeOf(args) {
	case ModeAPI:
		return runAPI(args[1:], stderr)
	case ModeVerb:
		return runVerb(args, stdout, stderr)
	default:
		// ModeUsage, and anything a later mode adds to ModeOf without adding
		// itself here. Saying what the binary is for is the right answer to
		// both, and it is the wrong answer loudly rather than exit 0 quietly.
		return usage(stderr)
	}
}

// verbs is every verb there is, in the order usage names them, so the sentence
// a bare `tasks` prints and the dispatch below cannot name different sets.
var verbs = []struct {
	verb string
	run  func(args []string, stdout, stderr io.Writer) int
}{
	{"list", list},
	{"repos", listRepos},
	{"lint", lint},
	{"add", add},
	{"edit", edit},
	{"move", move},
	{"delete", remove},
	{"tags", tags},
}

// usage is bare `tasks`: what the binary does and how to reach it. It is an
// error rather than a help screen because nothing was asked for.
func usage(stderr io.Writer) int {
	named := make([]string, len(verbs))
	for i, one := range verbs {
		named[i] = one.verb
	}
	fmt.Fprintf(stderr, `tasks: a task tracker for a person and for agents.

  tasks <verb> [flags]   act and exit
  tasks api [flags]      serve the json and the browser client on the lan

Verbs: %s. Each takes -h for its own flags.
`, strings.Join(named, ", "))
	return 2
}

// runAPI is `tasks api`: the same reads over HTTP, for the browser client. It
// serves the compiled client's files beside the JSON when it is given a
// directory of them, so there is no second process and no CORS.
//
// There is no authentication, deliberately: the listener is for the LAN, and
// ADR 0003's first re-check trigger is what covers changing that.
func runAPI(args []string, stderr io.Writer) int {
	fs := flags("api", stderr)
	var o api.Options
	fs.StringVar(&o.Addr, "addr", ":8080", "address to listen on")
	fs.StringVar(&o.Web, "web", "", "directory of compiled client files to serve beside the json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	o.Actor = actor()
	if err := api.ListenAndServe(o, stderr); err != nil {
		fmt.Fprintf(stderr, "tasks api: %v\n", err)
		return 1
	}
	return 0
}

// runVerb is the acts-and-exits mode. Each verb reads through the same board
// package the API reads through, so an Agent and a person get the same answer.
func runVerb(args []string, stdout, stderr io.Writer) int {
	verb, rest := args[0], args[1:]
	for _, one := range verbs {
		if one.verb == verb {
			return one.run(rest, stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "tasks: unknown verb %q\n", verb)
	return 2
}
