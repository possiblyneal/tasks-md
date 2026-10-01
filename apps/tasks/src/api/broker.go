package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/ai"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/write"
)

// The Broker routes. capture, ask and breakdown each read the board, make one
// turn and write nothing: a dump becomes a Task only when the add form is
// submitted, and a proposal only when subtasks is. The Broker is never an
// Actor.

// capture is POST /api/capture: a dump read into the body POST /api/tasks
// takes, with the Repo the Broker guessed. The attributes come back as said,
// because the form is where somebody corrects one; a Repo or Tag it named is
// one offered or nothing.
//
// A dump naming a Repo and a Task amends that Task: the Broker is handed it as
// it stands and answers with the whole of it, and the Repo is the Task's own,
// because an edit does not move a Task.
func capture(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
		Repo string `json:"repo"`
		Task string `json:"task"`
	}
	if err := strict(r, &body); err != nil {
		fail(w, err)
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		fail(w, usage{errors.New("say what the Task is")})
		return
	}
	read, err := everyRepo(o)
	if err != nil {
		fail(w, err)
		return
	}
	dump := write.DumpOf(text, read, o.now())
	if body.Task != "" {
		found, err := board.Named(body.Repo)
		if err != nil {
			fail(w, err)
			return
		}
		if dump.Was, err = write.WasOf(board.ReadRepo(found), body.Task); err != nil {
			fail(w, err)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), ai.Patience)
	defer cancel()
	said, err := ai.New().Read(ctx, dump)
	if err != nil {
		fail(w, err)
		return
	}
	repo := ""
	if guess := write.NamedIn([]string{said.Repo}, dump.Repos); len(guess) == 1 {
		repo = guess[0]
	}
	if dump.Was != nil {
		repo = dump.Was.Repo
	}
	send(w, http.StatusOK, struct {
		Repo string `json:"repo"`
		write.New
	}{repo, write.New{
		Title:       said.Title,
		Description: said.Description,
		Why:         said.Why,
		Deadline:    said.Deadline,
		Estimate:    said.Estimate,
		Priority:    said.Priority,
		Impact:      said.Impact,
		Tags:        write.NamedIn(said.Tags, dump.Tags),
	}})
}

// ask is POST /api/ask: a question about the Tasks the query narrows to, read
// the way GET /api/state reads it, answered in prose.
func ask(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Question string `json:"question"`
	}
	if err := strict(r, &body); err != nil {
		fail(w, err)
		return
	}
	question := strings.TrimSpace(body.Question)
	if question == "" {
		fail(w, usage{errors.New("say what the question is")})
		return
	}
	n, err := narrowing(r)
	if err != nil {
		fail(w, err)
		return
	}
	found, _ := board.Discover()
	only, err := board.Only(found, n.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	pass(o, only...)
	read, err := board.Read(found, n)
	if err != nil {
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), ai.Patience)
	defer cancel()
	answer, err := ai.New().Ask(ctx, question, write.Briefs(read))
	if err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusOK, map[string]string{"answer": answer})
}

// breakdown is POST /api/breakdown: one turn of breaking a Task down. The
// whole conversation comes with every turn, because the Broker keeps none of
// it. A proposal goes back with what the store would refuse already dropped,
// so a ticked one is written as it was shown.
func breakdown(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo    string  `json:"repo"`
		Task    string  `json:"task"`
		Answers []ai.QA `json:"answers"`
	}
	if _, err := decode(o, r, &body, &body.Repo); err != nil {
		fail(w, err)
		return
	}
	found, err := board.Named(body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	brief, err := write.BriefOf(board.ReadRepo(found), body.Task)
	if err != nil {
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), ai.Patience)
	defer cancel()
	step, err := ai.New().Breakdown(ctx, brief, body.Answers)
	if err != nil {
		fail(w, err)
		return
	}
	out := struct {
		Questions []string    `json:"questions,omitempty"`
		Proposals []write.New `json:"proposals"`
	}{Questions: step.Questions, Proposals: []write.New{}}
	for _, p := range step.Proposals {
		// One without a title could never be written, and ticking it would
		// refuse the whole batch.
		if n := write.FromProposal(p); n.Title != "" {
			out.Proposals = append(out.Proposals, n)
		}
	}
	// Neither half is the Broker having answered nothing usable: not this side
	// failing and not the caller asking wrongly, which is what 500 means here.
	if len(out.Questions) == 0 && len(out.Proposals) == 0 {
		fail(w, errors.New("the broker had nothing to ask and nothing to propose"))
		return
	}
	send(w, http.StatusOK, out)
}

// subtasks is POST /api/tasks/{id}/subtasks: the approved proposals, written
// under the Task as one commit by the listener's Actor.
func subtasks(o Options, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo     string      `json:"repo"`
		Version  string      `json:"version"`
		Subtasks []write.New `json:"subtasks"`
	}
	dir, err := decode(o, r, &body, &body.Repo)
	if err != nil {
		fail(w, err)
		return
	}
	ids, err := write.AddSubtasks(dir, o.Actor, r.PathValue("id"), body.Version, body.Subtasks)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, http.StatusCreated, map[string][]string{"ids": ids})
}

// strict reads a body that names no Repo, refusing a field it does not know.
func strict(r *http.Request, body any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		return usage{err}
	}
	return nil
}

// everyRepo is every Repo's Tasks, after the pass.
func everyRepo(o Options) ([]board.Repo, error) {
	found, _ := board.Discover()
	pass(o, found...)
	return board.Read(found, board.Narrowing{})
}
