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
