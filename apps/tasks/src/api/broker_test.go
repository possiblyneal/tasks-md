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
)

// broker stands in for inference-runtime-broker and answers every turn with
// what the test hands it. What it was told is kept, so a test can say what
// crossed the wire.
func broker(t *testing.T, answer string) *string {
	t.Helper()
	told := new(string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sent map[string]any
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("the request body was not JSON: %v", err)
		}
		turns, _ := json.Marshal(sent["messages"])
		*told = string(turns)
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": answer}}},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TASKS_AI_URL", srv.URL+"/v1")
	t.Setenv("TASKS_AI_MODEL", "a-model")
	return told
}

// unwritten fails the test when a Repo's TASKS.md has changed since files.
func unwritten(t *testing.T, code string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		got, err := os.ReadFile(filepath.Join(code, name, "TASKS.md"))
		if err != nil || string(got) != text {
			t.Errorf("%s/TASKS.md was written: %s", name, got)
		}
	}
}

func TestCaptureAnswersADraftWithTheRepoGuessAndWritesNothing(t *testing.T) {
	files := map[string]string{"house-move": houseMove, "work": work}
	code := homeWith(t, files)
	told := broker(t, `{"title":"Paint the fence","why":"it is peeling","deadline":"2026-03-04",
		"estimate":"90m","priority":"high","repo":"WORK","tags":["Kitchen","nothing by that name"]}`)
	h := Handler(Options{Actor: "neal"})

	w := post(t, h, "/api/capture", `{"text": "paint the fence before march"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/capture answered %d: %s", w.Code, w.Body.String())
	}
	var draft map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	// The Repo is the offered name it guessed, as offered. The attributes come
	// back as said, because the form is where somebody corrects one.
	for key, want := range map[string]string{"repo": "work", "title": "Paint the fence", "why": "it is peeling", "estimate": "90m", "priority": "high"} {
		if draft[key] != want {
			t.Errorf("the draft's %s = %v, want %q", key, draft[key], want)
		}
	}
	if tags, _ := json.Marshal(draft["tags"]); string(tags) != `["kitchen"]` {
		t.Errorf("the draft's tags = %s, want the offered one it chose", tags)
	}
	for _, want := range []string{"paint the fence before march", "house-move", "work", "kitchen"} {
		if !strings.Contains(*told, want) {
			t.Errorf("the Broker was told %s, want %q in it", *told, want)
		}
	}
	unwritten(t, code, files)
}

func TestAskAnswersAboutTheTasksInViewAndWritesNothing(t *testing.T) {
	files := map[string]string{"house-move": houseMove, "work": work}
	code := homeWith(t, files)
	told := broker(t, "The report is the only one.")
	h := Handler(Options{Actor: "neal"})

	w := post(t, h, "/api/ask?repo=work", `{"question": "what is left?"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/ask answered %d: %s", w.Code, w.Body.String())
	}
	var said struct{ Answer string }
	if err := json.Unmarshal(w.Body.Bytes(), &said); err != nil || said.Answer != "The report is the only one." {
		t.Errorf("POST /api/ask answered %s, want the Broker's prose", w.Body.String())
	}
	if !strings.Contains(*told, "Write the report") || strings.Contains(*told, "Pack the kitchen") {
		t.Errorf("the Broker was told %s, want only the Tasks in view", *told)
	}
	if !strings.Contains(*told, `\"repo\":\"work\"`) {
		t.Errorf("the Broker was told %s, want each Task's Repo by name", *told)
	}
	unwritten(t, code, files)

	for _, c := range []struct{ target, body string }{
		{"/api/ask", `{"question": " "}`},
		{"/api/ask?sort=sideways", `{"question": "what is left?"}`},
		{"/api/capture", `{"text": ""}`},
	} {
		if w := post(t, h, c.target, c.body); w.Code != http.StatusBadRequest {
			t.Errorf("POST %s %s answered %d, want 400", c.target, c.body, w.Code)
		}
	}
}

func TestBreakdownProposesWhatTheStoreTakesAndWritesNothing(t *testing.T) {
	files := map[string]string{"house-move": houseMove}
	code := homeWith(t, files)
	told := broker(t, `{"proposals":[{"title":"Sand it","estimate":"90m","priority":"med"},{"title":"Prime it","estimate":"small"}]}`)
	h := Handler(Options{Actor: "neal"})

	w := post(t, h, "/api/breakdown", `{"repo": "house-move", "task": "m3qa", "answers": [{"question": "Which room?", "answer": "Kitchen"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/breakdown answered %d: %s", w.Code, w.Body.String())
	}
	var step struct {
		Proposals []map[string]any
	}
	if err := json.Unmarshal(w.Body.Bytes(), &step); err != nil || len(step.Proposals) != 2 {
		t.Fatalf("POST /api/breakdown answered %s, want two proposals", w.Body.String())
	}
	// An estimate the store would refuse is dropped, so a ticked proposal is
	// written as it was shown.
	if _, ok := step.Proposals[0]["estimate"]; ok || step.Proposals[0]["priority"] != "med" || step.Proposals[1]["estimate"] != "small" {
		t.Errorf("the proposals came back %v", step.Proposals)
	}
	for _, want := range []string{"Pack the kitchen", "Which room?", "Kitchen"} {
		if !strings.Contains(*told, want) {
			t.Errorf("the Broker was told %s, want %q in it", *told, want)
		}
	}
	unwritten(t, code, files)

	if w := post(t, h, "/api/breakdown", `{"repo": "house-move", "task": "zzzz"}`); w.Code != http.StatusNotFound {
		t.Errorf("breaking down a Task not there answered %d, want 404", w.Code)
	}
	broker(t, `{}`)
	if w := post(t, h, "/api/breakdown", `{"repo": "house-move", "task": "m3qa"}`); w.Code != http.StatusInternalServerError {
		t.Errorf("a Broker answering nothing answered %d, want 500", w.Code)
	}
}

func TestApprovedSubtasksAreOneWriteAsTheListenersActor(t *testing.T) {
	code := homeWith(t, map[string]string{"house-move": houseMove})
	gitUser(t)
	h := Handler(Options{Actor: "neal"})
	dir := filepath.Join(code, "house-move")
	version := task(t, h, "house-move", "v9t1").Version

	w := post(t, h, "/api/tasks/v9t1/subtasks", `{"repo": "house-move", "version": "`+version+`",
		"subtasks": [{"title": "Call the van company"}, {"title": "Pay the deposit", "estimate": "small"}]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/tasks/v9t1/subtasks answered %d: %s", w.Code, w.Body.String())
	}
	var added struct{ IDs []string }
	if err := json.Unmarshal(w.Body.Bytes(), &added); err != nil || len(added.IDs) != 2 {
		t.Fatalf("the route answered %s, want two ids", w.Body.String())
	}
	if got, want := lastCommit(t, dir), "chore(tasks): add ^"+added.IDs[0]+" ^"+added.IDs[1]+" under ^v9t1|neal"; got != want {
		t.Errorf("the last commit = %q, want %q", got, want)
	}
	out, err := exec.Command("git", "-C", dir, "--git-dir=.tasks.git", "rev-list", "--count", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(out)) != "2" {
		t.Errorf("the tasks history holds %s commits, want the first commit and the one write", out)
	}
	for i, title := range []string{"Call the van company", "Pay the deposit"} {
		if got := task(t, h, "house-move", added.IDs[i]); got.Title != title || !slices.Equal(got.Parents, []string{"Book the van"}) {
			t.Errorf("^%s reads back as %+v", added.IDs[i], got)
		}
	}

	for _, c := range []struct {
		body   string
		status int
	}{
		{`{"repo": "house-move", "version": "` + version + `", "subtasks": [{"title": "Again"}]}`, http.StatusConflict},
		{`{"repo": "house-move", "subtasks": []}`, http.StatusBadRequest},
		{`{"repo": "house-move", "subtasks": [{"title": "Fine"}, {"title": ""}]}`, http.StatusBadRequest},
	} {
		if w := post(t, h, "/api/tasks/v9t1/subtasks", c.body); w.Code != c.status {
			t.Errorf("POST %s answered %d, want %d", c.body, w.Code, c.status)
		}
	}
	if w := post(t, h, "/api/tasks/zzzz/subtasks", `{"repo": "house-move", "subtasks": [{"title": "Lost"}]}`); w.Code != http.StatusNotFound {
		t.Errorf("Subtasks under a Task not there answered %d, want 404", w.Code)
	}
}
