package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/history"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/repos"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// state is GET /api/state: every Repo's Tasks, narrowed by the query under the
// names `tasks list` takes its flags under, each Repo carrying its problems.
// The Repos are rescanned and every file re-read on each request, so a new
// Repo or a pull shows without a restart.
func state(o Options, w http.ResponseWriter, r *http.Request) {
	// The query is refused before the ETag is looked at, because the ETag
	// block answers without reading: `If-None-Match: *` under a State there is
	// not would otherwise be told nothing changed about a view it can never
	// be shown.
	n, err := narrowing(r)
	if err != nil {
		fail(w, err)
		return
	}

	found, errs := board.Discover()
	// An unknown Repo is refused before the ETag for the reason a bad query is.
	only, err := board.Only(found, n.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	// The pass goes ahead of the ETag, because what it writes changes the files.
	pass(o, only...)
	tag := version(found, errs, r.URL.RawQuery)
	if matches(r.Header.Values("If-None-Match"), tag) {
		w.Header().Set("ETag", tag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	read, err := board.Read(found, n)
	if err != nil {
		fail(w, err)
		return
	}
	weeks, err := board.Weekly(found, n, o.now())
	if err != nil {
		fail(w, err)
		return
	}
	// The ETag goes on the response that was actually sent, never on an error.
	w.Header().Set("ETag", tag)
	send(w, http.StatusOK, board.Of(read, weeks, errs))
}

// narrowing reads a Narrowing out of the query. A repeated parameter is a set,
// and an empty one narrows nothing.
func narrowing(r *http.Request) (board.Narrowing, error) {
	q := r.URL.Query()
	n := board.Narrowing{Repo: q.Get("repo"), Search: q.Get("search"), Sort: board.Sort(q.Get("sort"))}
	for _, s := range q["state"] {
		if s != "" {
			n.States = append(n.States, taskfile.State(s))
		}
	}
	for _, tag := range q["tag"] {
		if tag != "" {
			n.Tags = append(n.Tags, tag)
		}
	}
	if asked := q.Get("unblocked"); asked != "" {
		unblocked, err := strconv.ParseBool(asked)
		if err != nil {
			return n, usage{fmt.Errorf("cannot read unblocked=%q: want true or false", asked)}
		}
		n.Unblocked = unblocked
	}
	if err := n.Check(); err != nil {
		return n, usage{err}
	}
	return n, nil
}

// version is the ETag: every Repo found, its file's modification time and
// size, its flags, the config errors, the query and the host's date, hashed.
// Any of them changing is a different response, so none can be answered 304
// for another: past midnight a Deadline that was today is Overdue. A file that
// cannot be stat'd hashes as such and is read, and reported, by the read.
func version(found []repos.Repo, errs []error, query string) string {
	h := sha256.New()
	fmt.Fprintln(h, board.Today())
	for _, r := range found {
		fmt.Fprintf(h, "%s\x00%s\x00", r.Name, r.Path)
		if info, err := os.Stat(r.File()); err == nil {
			fmt.Fprintf(h, "%d\x00%d", info.ModTime().UnixNano(), info.Size())
		}
		// A push going through changes no file, and is still news.
		fmt.Fprintln(h, "\x00"+strings.Join(history.Flags(r.Path), "\x00"))
	}
	for _, err := range errs {
		fmt.Fprintln(h, err)
	}
	fmt.Fprint(h, query)
	return `"` + hex.EncodeToString(h.Sum(nil)) + `"`
}

// matches reads If-None-Match the way RFC 9110 writes it: several tags to one
// header line separated by commas, a weak tag marked `W/`, and `*` for any
// representation at all. Comparison is weak, which is what a GET of an
// unchanged response wants; only a range request needs the strong kind.
func matches(values []string, tag string) bool {
	for _, value := range values {
		for _, one := range strings.Split(value, ",") {
			one = strings.TrimSpace(one)
			if one == "*" || strings.TrimPrefix(one, "W/") == tag {
				return true
			}
		}
	}
	return false
}
