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

// decode reads a write's body, refusing a field it does not know so a
// misspelt attribute is not dropped quietly, and finds the Repo it names with
// the pass run over it.
func decode(o Options, r *http.Request, body any, repo *string) (string, error) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		return "", usage{err}
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
