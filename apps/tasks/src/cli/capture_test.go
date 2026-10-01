package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// standIn is the Broker, played by an httptest server, and TASKS_AI_URL is
// how the verb is pointed at it. No test here reaches the LAN.
func standIn(t *testing.T, content string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TASKS_AI_URL", srv.URL+"/v1")
	t.Setenv("TASKS_AI_MODEL", "stand-in")
}

// The Broker guesses work; the dump was said in house-move, which is where it
// goes. Unreadable values are dropped, and a Tag nobody has is not created.
const dentist = `{"title":"Call the dentist","description":"about the crown",
	"deadline":"2026-11-02 11:00","estimate":"20m","priority":"high",
	"repo":"work","tags":["Kitchen","made up"]}`

func TestCaptureWritesToTheCallingRepoNotTheBrokersGuess(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove, "work": work})
	standIn(t, dentist)
	house := filepath.Join(code, "house-move")
	t.Chdir(house)

	status, out, errs := run(t, "capture", "dentist", "about", "the", "crown")
	if status != 0 {
		t.Fatalf("tasks capture exited %d: %s", status, errs)
	}
	id := strings.TrimSpace(out)
	if !anID.MatchString(id) {
		t.Fatalf("tasks capture printed %q, want the new id", out)
	}
	want := "- [ ] Call the dentist | inbox #kitchen\n  - id: " + id + "\n  - created: " + today() + "\n  - priority: high\n\n  about the crown\n"
	if file := read(t, filepath.Join(house, "tasks.md")); !strings.Contains(file, want) {
		t.Errorf("house-move/tasks.md =\n%s\nwant it to hold\n%s", file, want)
	}
	if file := read(t, filepath.Join(code, "work", "tasks.md")); file != work {
		t.Errorf("work/tasks.md was written:\n%s", file)
	}
	if got := tasksLog(t, house, "%s|%(trailers:key=Generated-By,valueonly,separator=)")[0]; got != "chore(tasks): add ^"+id+" Call the dentist|claude-code/claude-opus-5-5" {
		t.Errorf("the capture's commit = %q, want it made as the caller's Actor", got)
	}

	// -repo names where it goes, still never the Broker's guess.
	t.Chdir(code)
	if status, _, errs := run(t, "capture", "-repo", "house-move", "dentist again"); status != 0 {
		t.Fatalf("tasks capture -repo exited %d: %s", status, errs)
	}
	if file := read(t, filepath.Join(code, "work", "tasks.md")); file != work {
		t.Errorf("work/tasks.md was written:\n%s", file)
	}
}

// -dry prints what was read and writes nothing.
func TestCaptureDryWritesNothing(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	standIn(t, dentist)

	status, out, errs := run(t, "capture", "-dry", "-repo", "house-move", "dentist about the crown")
	if status != 0 {
		t.Fatalf("tasks capture -dry exited %d: %s", status, errs)
	}
	for _, want := range []string{"Call the dentist", "about the crown", "work"} {
		if !strings.Contains(out, want) {
			t.Errorf("-dry printed %q, want %q in it", out, want)
		}
	}
	if file := read(t, filepath.Join(code, "house-move", "tasks.md")); file != houseMove {
		t.Errorf("-dry wrote tasks.md:\n%s", file)
	}
}

func TestCaptureRefusesNothingToRead(t *testing.T) {
	gitHome(t, map[string]string{"house-move": houseMove})
	standIn(t, `{"description":"no title here"}`)

	if status, _, _ := run(t, "capture", "-repo", "house-move"); status != 2 {
		t.Errorf("tasks capture with no words exited %d, want 2", status)
	}
	if status, _, errs := run(t, "capture", "-repo", "house-move", "hmm"); status != 1 || !strings.Contains(errs, "no title") {
		t.Errorf("a dump read into no title exited %d: %s, want 1 saying so", status, errs)
	}
}
