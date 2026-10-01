// Package api is the JSON surface the browser client reads the tracker
// through. It holds routing, encoding and the status code an error takes, and
// no rules: every handler is another caller of the same board read the verbs
// call, so a person on a phone and an Agent at a terminal get the same
// contract. GET /api/files is the one exception and says so where it is
// registered: it reads a directory rather than the Repos, so the root it will
// not look above is a rule with nowhere else to live.
//
// See docs/adrs/0003-replace-the-tui-with-a-browser-client.md and
// docs/plans/browser-client.md.
package api

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/repos"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/write"
)

// Options is how the listener is configured. Web is the directory of compiled
// client files served beside the JSON; empty serves the JSON alone, which is
// what a client running its own dev server wants.
//
// Browse is the directory the file picker lists from and will not look above.
// Empty is the home directory of the user running `tasks api`, which is the
// machine a pointer's relative path resolves against.
//
// Actor is who every write through the listener is made as, resolved once
// when it starts.
//
// Now is the host's clock the pass reads today from, and After the timer the
// midnight pass waits on; nil is the real ones. Like Browse, they are test
// seams rather than operator knobs.
type Options struct {
	Addr   string
	Web    string
	Browse string
	Actor  string
	Now    func() time.Time
	After  func(time.Duration) <-chan time.Time
}

func (o Options) now() time.Time {
	if o.Now == nil {
		return time.Now()
	}
	return o.Now()
}

func (o Options) after(d time.Duration) <-chan time.Time {
	if o.After == nil {
		return time.After(d)
	}
	return o.After(d)
}

// Handler is every route, and it is what a test exercises without a listener.
func Handler(o Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		state(o, w, r)
	})
	mux.HandleFunc("POST /api/tasks", func(w http.ResponseWriter, r *http.Request) {
		add(o, w, r)
	})
	mux.HandleFunc("POST /api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		edit(o, w, r)
	})
	mux.HandleFunc("POST /api/tasks/{id}/move", func(w http.ResponseWriter, r *http.Request) {
		move(o, w, r)
	})
	mux.HandleFunc("GET /api/tasks/{id}/series", func(w http.ResponseWriter, r *http.Request) {
		series(o, w, r)
	})
	mux.HandleFunc("POST /api/tasks/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		remove(o, w, r)
	})
	mux.HandleFunc("POST /api/tags/rename", func(w http.ResponseWriter, r *http.Request) {
		rename(o, w, r)
	})
	// The one route reading outside the Repos. It lists and never opens, and
	// what it lists is the machine a pointer resolves against rather than the
	// phone the pointer is being typed into.
	root := rooted(cmp.Or(o.Browse, home()))
	mux.HandleFunc("GET /api/files", func(w http.ResponseWriter, r *http.Request) {
		browse(root, w, r)
	})
	// A route under /api/ that this package does not serve is a usage error in
	// the same envelope every other one arrives in. It is registered whether or
	// not the client is served here: with the files under it the fallback below
	// would otherwise hand back index.html at 200 for the client to fail to
	// parse as JSON, and without them a bad route would answer this in one
	// posture and Go's own plain-text 404 in the other.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, usage{fmt.Errorf("%s %s is not a route", r.Method, r.URL.Path)})
	})
	if o.Web != "" {
		mux.Handle("/", client(o.Web))
	}
	return mux
}

// ListenAndServe opens the listener. The address is the caller's, and ADR
// 0003 binds it to the LAN: there is no authentication here on purpose, so an
// address reachable from outside the LAN is a decision that record's first
// re-check trigger covers.
func ListenAndServe(o Options, stderr io.Writer) error {
	fmt.Fprintf(stderr, "tasks api: listening on %s\n", o.Addr)
	srv := &http.Server{Addr: o.Addr, Handler: Handler(o)}
	go nightly(o, nil)
	return srv.ListenAndServe()
}

// nightly runs the pass over every Repo one second past each local midnight,
// so a file is a day's work behind at most even when nothing reads it, until
// stop is closed.
func nightly(o Options, stop <-chan struct{}) {
	for {
		now := o.now()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, now.Location())
		select {
		case <-stop:
			return
		case <-o.after(midnight.Sub(now)):
		}
		found, _ := board.Discover()
		pass(o, found...)
	}
}

// pass runs the pass over the Repos a request is about to read or write.
func pass(o Options, found ...repos.Repo) {
	dirs := make([]string, len(found))
	for i, r := range found {
		dirs[i] = r.Path
	}
	write.Pass(o.now(), dirs...)
}

// client serves the compiled client, falling back to index.html for a path
// that names no file. The client routes in the browser, so a reload on a path
// only it knows about has to reach it rather than 404.
func client(dir string) http.Handler {
	root := http.Dir(dir)
	files := http.FileServer(root)
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The same http.Dir decides whether the file is there and then serves
		// it, so the two cannot disagree about which file a request names, and
		// the one check that keeps a request inside the directory is the
		// standard library's rather than a second one written here to match.
		file, err := root.Open(r.URL.Path)
		if err != nil {
			http.ServeFile(w, r, index)
			return
		}
		info, err := file.Stat()
		_ = file.Close()
		if err != nil || info.IsDir() {
			http.ServeFile(w, r, index)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// send writes one JSON body. An encoding failure after the status is written
// cannot be reported to the client, so it is logged nowhere and dropped: the
// response is already short by then and the client's next poll replaces it.
func send(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// fail maps an error onto the status carrying the meaning the CLI's exit
// status carries: a bad request is not a failure, neither is a missing Repo or
// Task, and a write the rules turn away is a conflict.
// The body is the sentence the CLI would have printed.
func fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var asked usage
	var invalid write.Invalid
	var absent board.NoRepo
	var noTask write.NoTask
	switch {
	case write.IsRefused(err):
		status = http.StatusConflict
	case errors.As(err, &asked), errors.As(err, &invalid):
		status = http.StatusBadRequest
	case errors.As(err, &absent), errors.As(err, &noTask):
		status = http.StatusNotFound
	}
	send(w, status, map[string]string{"error": err.Error()})
}

// usage marks an error the caller can fix by asking differently, which is exit
// status 2 at a terminal and 400 here. It adds no words of its own: the body
// is the sentence the CLI would have printed and nothing more, so a person
// reading the browser and a person reading the terminal are told the same
// thing about the same mistake.
type usage struct{ err error }

func (u usage) Error() string { return u.err.Error() }
func (u usage) Unwrap() error { return u.err }
