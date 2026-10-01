package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/write"
)

// add is POST /api/tasks: `tasks add` over HTTP. The body is a write.New with
// the Repo named in it, and the answer is 201 with the new id.
func add(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo string `json:"repo"`
		write.New
	}
	dir, err := decode(o, r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id, err := write.Add(dir, o.Actor, body.New)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusCreated, map[string]string{"id": id})
}

// move is POST /api/tasks/{id}/move: `tasks move` over HTTP, with the body
// {repo, state, reason?}.
func move(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo   string `json:"repo"`
		State  string `json:"state"`
		Reason string `json:"reason"`
	}
	dir, err := decode(o, r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("id")
	if err := write.Move(dir, o.Actor, id, taskfile.State(body.State), body.Reason); err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"id": id})
}

// edit is POST /api/tasks/{id}: `tasks edit` over HTTP. The body is a
// write.Edit with the Repo named in it, a field left out is left alone, and
// the answer is {"id"}.
func edit(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo string `json:"repo"`
		write.Edit
	}
	dir, err := decode(o, r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("id")
	if err := write.EditTask(dir, o.Actor, id, body.Edit); err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"id": id})
}

// remove is POST /api/tasks/{id}/delete: `tasks delete` over HTTP, with the
// body {repo, version?}.
func remove(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo    string `json:"repo"`
		Version string `json:"version"`
	}
	dir, err := decode(o, r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("id")
	if err := write.Delete(dir, o.Actor, id, body.Version); err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"id": id})
}

// rename is POST /api/tags/rename: `tasks tags rename` over HTTP across every
// Repo, with the body {from, to, versions?}, versions being the trees the
// caller read. The answer names the Repos written.
func rename(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		From     string   `json:"from"`
		To       string   `json:"to"`
		Versions []string `json:"versions"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		fail(w, usage{err})
		return
	}
	found, _ := board.Discover()
	pass(o, found...)
	dirs := make([]string, len(found))
	names := map[string]string{}
	for i, f := range found {
		dirs[i], names[f.Path] = f.Path, f.Name
	}
	written, err := write.RenameTag(dirs, o.Actor, body.From, body.To, body.Versions)
	if err != nil {
		fail(w, err)
		return
	}
	repos := make([]string, len(written))
	for i, dir := range written {
		repos[i] = names[dir]
	}
	send(w, http.StatusOK, map[string][]string{"repos": repos})
}

// decode reads a write's body, refusing a field it does not know so a
// misspelt attribute is not dropped quietly, and finds the Repo it names with
// the pass run over it.
func decode(o Options, r *http.Request, body any, repo *string) (string, error) {
	if err := strict(r, body); err != nil {
		return "", err
	}
	if *repo == "" {
		return "", usage{errors.New("a write names its Repo in the body's repo")}
	}
	return named(o, *repo)
}

// named is the folder of the Repo a request names, with the pass run over it.
func named(o Options, repo string) (string, error) {
	found, err := board.Named(repo)
	if err != nil {
		return "", err
	}
	pass(o, found)
	return found.Path, nil
}

// series is GET /api/tasks/{id}/series?repo=<name>: `tasks repeat <id>` over
// HTTP, the rule and its next dates from today. It writes nothing but the pass.
func series(o Options, w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		fail(w, usage{errors.New("a Series is asked for with its Repo in the query's repo")})
		return
	}
	dir, err := named(o, repo)
	if err != nil {
		fail(w, err)
		return
	}
	s, err := write.SeriesOf(dir, r.PathValue("id"), o.now())
	if err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, s)
}

// taskHistory is GET /api/tasks/{id}/history?repo=<name>: what happened to the
// Task, newest first, as `{"entries": [{at, actor, subject}]}`. It writes
// nothing but the pass.
func taskHistory(o Options, w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		fail(w, usage{errors.New("a Task's history is asked for with its Repo in the query's repo")})
		return
	}
	dir, err := named(o, repo)
	if err != nil {
		fail(w, err)
		return
	}
	entries, err := write.HistoryOf(dir, r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string][]write.Entry{"entries": entries})
}
