package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// gitUser gives the test's home a host git user for the tasks history's
// commits to be authored as.
func gitUser(t *testing.T) {
	t.Helper()
	config := "[user]\n\tname = Host Person\n\temail = host@example.com\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func post(t *testing.T, h http.Handler, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, strings.NewReader(body)))
	return w
}

func TestAddAndMoveAnswerWithTheCLIsStatuses(t *testing.T) {
	code := homeWith(t, map[string]string{"house-move": houseMove})
	gitUser(t)
	h := Handler(Options{Actor: "neal"})

	w := post(t, h, "/api/tasks", `{"repo": "house-move", "title": "Measure the hallway", "tags": ["measuring"], "priority": "high"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/tasks answered %d: %s", w.Code, w.Body.String())
	}
	var added struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &added); err != nil || len(added.ID) != 4 {
		t.Fatalf("POST /api/tasks answered %s, want the new id", w.Body.String())
	}

	house := decodeState(t, get(t, h, "/api/state", nil)).Repos[0]
	i := slices.IndexFunc(house.Tasks, func(task board.Task) bool { return task.ID == added.ID })
	if i < 0 || house.Tasks[i].State != "inbox" || !slices.Contains(house.Tasks[i].Attrs, taskfile.Attr{Label: "priority", Value: "high"}) {
		t.Errorf("the added Task reads back as %+v", house.Tasks)
	}
	if !slices.Equal(house.Flags, []string{"not backed up"}) {
		t.Errorf("house-move flags = %q, want not backed up", house.Flags)
	}

	if w := post(t, h, "/api/tasks/v9t1/move", `{"repo": "house-move", "state": "doing"}`); w.Code != http.StatusOK {
		t.Fatalf("moving v9t1 to doing answered %d: %s", w.Code, w.Body.String())
	}
	out, err := exec.Command("git", "-C", filepath.Join(code, "house-move"), "--git-dir=.tasks.git",
		"log", "-1", "--format=%s|%(trailers:key=Generated-By,valueonly,separator=)").Output()
	if err != nil || strings.TrimSpace(string(out)) != "chore(tasks): move ^v9t1 to Doing|neal" {
		t.Errorf("the move's commit = %q, %v; want it made as the listener's Actor", out, err)
	}

	for _, c := range []struct {
		target, body string
		status       int
	}{
		{"/api/tasks/v9t1/move", `{"repo": "house-move", "state": "doing"}`, http.StatusConflict},
		{"/api/tasks/m3qa/move", `{"repo": "house-move", "state": "done"}`, http.StatusConflict},
		{"/api/tasks/zzzz/move", `{"repo": "house-move", "state": "done"}`, http.StatusNotFound},
		{"/api/tasks/v9t1/move", `{"repo": "nowhere", "state": "done"}`, http.StatusNotFound},
		{"/api/tasks/v9t1/move", `{"repo": "house-move", "state": "finished"}`, http.StatusBadRequest},
		{"/api/tasks/v9t1/move", `{"state": "done"}`, http.StatusBadRequest},
		{"/api/tasks", `{"repo": "house-move", "title": "Paint", "colour": "red"}`, http.StatusBadRequest},
		{"/api/tasks", `{"repo": "house-move", "title": ""}`, http.StatusBadRequest},
	} {
		w := post(t, h, c.target, c.body)
		var body struct{ Error string }
		if w.Code != c.status || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Error == "" {
			t.Errorf("POST %s %s answered %d %s, want %d with a sentence", c.target, c.body, w.Code, w.Body.String(), c.status)
		}
	}
}

// task is the Task with the id in the Repo's read, failing the test without it.
func task(t *testing.T, h http.Handler, repo, id string) board.Task {
	t.Helper()
	for _, r := range decodeState(t, get(t, h, "/api/state", nil)).Repos {
		if r.Name != repo {
			continue
		}
		if i := slices.IndexFunc(r.Tasks, func(task board.Task) bool { return task.ID == id }); i >= 0 {
			return r.Tasks[i]
		}
	}
	t.Fatalf("%s holds no ^%s", repo, id)
	return board.Task{}
}

func TestEveryAttributeRoundTripsThroughAddAndEdit(t *testing.T) {
	homeWith(t, map[string]string{"house-move": houseMove})
	gitUser(t)
	h := Handler(Options{Actor: "neal"})

	w := post(t, h, "/api/tasks", `{"repo": "house-move", "title": "Paint the hall", "state": "deferred",
		"tags": ["paint"], "description": "Two coats.", "why": "it is scuffed", "acceptance": "no marks",
		"deadline": "2026-12-01", "priority": "high", "impact": "low", "estimate": "large", "color": "red",
		"blockedBy": ["v9t1"], "reason": "after the move", "until": "2026-11-01", "attach": ["~/paint.pdf"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/tasks answered %d: %s", w.Code, w.Body.String())
	}
	var added struct{ ID string }
	_ = json.Unmarshal(w.Body.Bytes(), &added)
	want := []taskfile.Attr{
		{Label: "deadline", Value: "2026-12-01"}, {Label: "priority", Value: "high"}, {Label: "impact", Value: "low"},
		{Label: "estimate", Value: "large"}, {Label: "color", Value: "red"}, {Label: "why", Value: "it is scuffed"},
		{Label: "acceptance", Value: "no marks"}, {Label: "blocked by", Value: "v9t1"}, {Label: "until", Value: "2026-11-01"},
		{Label: "reason", Value: "after the move"}, {Label: "attachment", Value: "~/paint.pdf"},
	}
	got := task(t, h, "house-move", added.ID)
	if !slices.Equal(got.Attrs, want) || got.State != "deferred" || got.Description != "Two coats." || !slices.Equal(got.Tags, []string{"paint"}) {
		t.Errorf("the add reads back as %+v\nwant attrs %+v", got, want)
	}

	w = post(t, h, "/api/tasks/"+added.ID, `{"repo": "house-move", "version": "`+got.Version+`", "title": "Paint the stairs",
		"tags": ["paint", "stairs"], "description": "One coat.", "why": "", "acceptance": "even", "deadline": "2027-01-01",
		"priority": "low", "impact": "high", "estimate": "small", "color": "blue", "blockedBy": [], "reason": "later",
		"until": "2026-11-02", "attach": ["~/stairs.pdf"], "detach": ["~/paint.pdf"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/tasks/%s answered %d: %s", added.ID, w.Code, w.Body.String())
	}
	want = []taskfile.Attr{
		{Label: "deadline", Value: "2027-01-01"}, {Label: "priority", Value: "low"}, {Label: "impact", Value: "high"},
		{Label: "estimate", Value: "small"}, {Label: "color", Value: "blue"}, {Label: "acceptance", Value: "even"},
		{Label: "until", Value: "2026-11-02"}, {Label: "reason", Value: "later"}, {Label: "attachment", Value: "~/stairs.pdf"},
	}
	got = task(t, h, "house-move", added.ID)
	if !slices.Equal(got.Attrs, want) || got.Title != "Paint the stairs" || got.Description != "One coat." || !slices.Equal(got.Tags, []string{"paint", "stairs"}) {
		t.Errorf("the edit reads back as %+v\nwant attrs %+v", got, want)
	}
}

func TestAStaleWriteIsAConflictAndAnotherTreeIsNot(t *testing.T) {
	code := homeWith(t, map[string]string{"house-move": houseMove})
	gitUser(t)
	h := Handler(Options{Actor: "neal"})
	kitchen, van := task(t, h, "house-move", "m3qa").Version, task(t, h, "house-move", "v9t1").Version

	if w := post(t, h, "/api/tasks/m3qc", `{"repo": "house-move", "version": "`+kitchen+`", "priority": "high"}`); w.Code != http.StatusOK {
		t.Fatalf("the first edit answered %d: %s", w.Code, w.Body.String())
	}
	for _, c := range []struct{ target, body string }{
		{"/api/tasks/m3qa", `{"repo": "house-move", "version": "` + kitchen + `", "title": "Pack it all"}`},
		{"/api/tasks/m3qa/delete", `{"repo": "house-move", "version": "` + kitchen + `"}`},
		{"/api/tags/rename", `{"from": "kitchen", "to": "packing", "versions": ["` + kitchen + `"]}`},
	} {
		if w := post(t, h, c.target, c.body); w.Code != http.StatusConflict {
			t.Errorf("POST %s against a stale tree answered %d: %s", c.target, w.Code, w.Body.String())
		}
	}
	if w := post(t, h, "/api/tasks/v9t1", `{"repo": "house-move", "version": "`+van+`", "title": "Book the big van"}`); w.Code != http.StatusOK {
		t.Errorf("an edit to another tree answered %d: %s", w.Code, w.Body.String())
	}

	fresh := task(t, h, "house-move", "m3qa").Version
	if w := post(t, h, "/api/tags/rename", `{"from": "kitchen", "to": "packing", "versions": ["`+fresh+`"]}`); w.Code != http.StatusOK {
		t.Errorf("a rename against what it read answered %d: %s", w.Code, w.Body.String())
	}
	if got := task(t, h, "house-move", "m3qa").Tags; !slices.Equal(got, []string{"packing"}) {
		t.Errorf("after the rename m3qa carries %q", got)
	}
	if w := post(t, h, "/api/tasks/m3qa/delete", `{"repo": "house-move"}`); w.Code != http.StatusOK {
		t.Errorf("a delete answered %d: %s", w.Code, w.Body.String())
	}
	text, _ := os.ReadFile(filepath.Join(code, "house-move", "tasks.md"))
	if strings.Contains(string(text), "m3q") {
		t.Errorf("the delete left\n%s", text)
	}

	for _, c := range []struct {
		target, body string
		status       int
	}{
		{"/api/tasks/v9t1", `{"repo": "house-move", "colour": "red"}`, http.StatusBadRequest},
		{"/api/tasks/v9t1", `{"title": "x"}`, http.StatusBadRequest},
		{"/api/tasks/zzzz", `{"repo": "house-move", "title": "x"}`, http.StatusNotFound},
		{"/api/tasks/zzzz/delete", `{"repo": "house-move"}`, http.StatusNotFound},
		{"/api/tags/rename", `{"from": "kitchen", "to": "two words"}`, http.StatusBadRequest},
	} {
		if w := post(t, h, c.target, c.body); w.Code != c.status {
			t.Errorf("POST %s %s answered %d %s, want %d", c.target, c.body, w.Code, w.Body.String(), c.status)
		}
	}
}
