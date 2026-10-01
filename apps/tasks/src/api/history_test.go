package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// entries is GET /api/tasks/{id}/history's answer as "actor: subject" lines,
// newest first, with every entry checked to carry the moment it was made.
func entries(t *testing.T, h http.Handler, target string) []string {
	t.Helper()
	w := get(t, h, target, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s answered %d %s", target, w.Code, w.Body.String())
	}
	var body struct {
		Entries []struct {
			At      time.Time
			Actor   string
			Subject string
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Entries == nil {
		t.Fatalf("GET %s answered %s, want {entries: [...]}", target, w.Body.String())
	}
	lines := []string{}
	for _, e := range body.Entries {
		if e.At.IsZero() {
			t.Errorf("GET %s: %+v carries no moment", target, e)
		}
		lines = append(lines, e.Actor+": "+e.Subject)
	}
	return lines
}

func TestATasksHistoryNamesEachWritesActorNewestFirst(t *testing.T) {
	code := homeWith(t, map[string]string{"house-move": houseMove})
	gitUser(t)
	agent := Handler(Options{Actor: "claude-code/opus"})
	person := Handler(Options{Actor: "neal"})

	if w := post(t, agent, "/api/tasks/v9t1/move", `{"repo": "house-move", "state": "doing"}`); w.Code != http.StatusOK {
		t.Fatalf("the Agent's move answered %d: %s", w.Code, w.Body.String())
	}
	if w := post(t, person, "/api/tasks/v9t1", `{"repo": "house-move", "priority": "high"}`); w.Code != http.StatusOK {
		t.Fatalf("the person's edit answered %d: %s", w.Code, w.Body.String())
	}

	got := entries(t, person, "/api/tasks/v9t1/history?repo=house-move")
	want := []string{
		"neal: edit ^v9t1 Book the van",
		"claude-code/opus: move ^v9t1 to Doing",
		// The file as it stood before the tracker first wrote it was made by
		// hand, so it is the first direct edit of every Task in it.
		": commit a direct edit",
	}
	if !slices.Equal(got, want) {
		t.Errorf("v9t1's history = %q, want %q", got, want)
	}

	// A hand edit to one Task is drawn on that Task alone, once a write has
	// committed it.
	dir := filepath.Join(code, "house-move")
	text, err := os.ReadFile(filepath.Join(dir, "TASKS.md"))
	if err != nil {
		t.Fatal(err)
	}
	place(t, dir, strings.Replace(string(text), "Wrap glassware", "Wrap the glassware", 1))
	if w := post(t, person, "/api/tasks", `{"repo": "house-move", "title": "Measure the hallway"}`); w.Code != http.StatusCreated {
		t.Fatalf("the add answered %d: %s", w.Code, w.Body.String())
	}

	if got := entries(t, person, "/api/tasks/m3qc/history?repo=house-move"); !slices.Equal(got, []string{": commit a direct edit", ": commit a direct edit"}) {
		t.Errorf("m3qc's history = %q, want the hand edit over the first one", got)
	}
	if got := entries(t, person, "/api/tasks/v9t1/history?repo=house-move"); !slices.Equal(got, want) {
		t.Errorf("v9t1's history after a hand edit to m3qc = %q, want it unchanged: %q", got, want)
	}
}

func TestATaskInARepoWithNoHistoryHasAnEmptyOne(t *testing.T) {
	homeWith(t, map[string]string{"house-move": houseMove})
	h := Handler(Options{})

	if got := entries(t, h, "/api/tasks/v9t1/history?repo=house-move"); len(got) != 0 {
		t.Errorf("history = %q, want none", got)
	}
	for target, status := range map[string]int{
		"/api/tasks/zzzz/history?repo=house-move": http.StatusNotFound,
		"/api/tasks/v9t1/history?repo=nowhere":    http.StatusNotFound,
		"/api/tasks/v9t1/history":                 http.StatusBadRequest,
	} {
		w := get(t, h, target, nil)
		var body struct{ Error string }
		if w.Code != status || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Error == "" {
			t.Errorf("GET %s answered %d %s, want %d with a sentence", target, w.Code, w.Body.String(), status)
		}
	}
}
