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

// edit is POST /api/tasks/{id}: `tasks edit` over HTTP. The body is a
// write.Edit with the Repo named in it, a field left out is left alone, and
// the answer is {"id"}.
func edit(actor string, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo string `json:"repo"`
		write.Edit
	}
	dir, err := decode(r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("id")
	if err := write.EditTask(dir, actor, id, body.Edit); err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"id": id})
}

// remove is POST /api/tasks/{id}/delete: `tasks delete` over HTTP, with the
// body {repo, version?}.
func remove(actor string, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo    string `json:"repo"`
		Version string `json:"version"`
	}
	dir, err := decode(r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("id")
	if err := write.Delete(dir, actor, id, body.Version); err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"id": id})
}

// rename is POST /api/tags/rename: `tasks tags rename` over HTTP across every
// Repo, with the body {from, to, versions?}, versions being the trees the
// caller read. The answer names the Repos written.
func rename(actor string, w http.ResponseWriter, r *http.Request) {
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
	dirs := make([]string, len(found))
	names := map[string]string{}
	for i, f := range found {
		dirs[i], names[f.Path] = f.Path, f.Name
	}
	written, err := write.RenameTag(dirs, actor, body.From, body.To, body.Versions)
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
