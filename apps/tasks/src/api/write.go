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
func add(actor string, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo string `json:"repo"`
		write.New
	}
	dir, err := decode(r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id, err := write.Add(dir, actor, body.New)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusCreated, map[string]string{"id": id})
}

// move is POST /api/tasks/{id}/move: `tasks move` over HTTP, with the body
// {repo, state, reason?}.
func move(actor string, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo   string `json:"repo"`
		State  string `json:"state"`
		Reason string `json:"reason"`
	}
	dir, err := decode(r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("id")
	if err := write.Move(dir, actor, id, taskfile.State(body.State), body.Reason); err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"id": id})
}

// decode reads a write's body, refusing a field it does not know so a
// misspelt attribute is not dropped quietly, and finds the Repo it names.
func decode(r *http.Request, body any, repo *string) (string, error) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		return "", usage{err}
	}
	if *repo == "" {
		return "", usage{errors.New("a write names its Repo in the body's repo")}
	}
	found, err := board.Named(*repo)
	return found.Path, err
}
