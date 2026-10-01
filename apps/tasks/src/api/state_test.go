package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
)

const houseMove = `# Tasks
color: green

- [ ] Pack the kitchen | doing #kitchen
  - id: m3qa
  - created: 2026-09-20
  - [ ] Wrap glassware | doing
    - id: m3qc
    - created: 2026-09-20
- [ ] Book the van | backlog
  - id: v9t1
  - created: 2026-09-21
  - blocked by: m3qa
`

const work = `# Tasks

- [ ] Write the report | inbox
  - created: 2026-09-22
`

// homeWith gives the test a home of its own whose one root, ~/code, holds a
// folder per entry with that tasks.md. It returns ~/code.
func homeWith(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	code := filepath.Join(dir, "code")
	for name, text := range files {
		place(t, filepath.Join(code, name), text)
	}
	return code
}

func place(t *testing.T, dir, text string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, h http.Handler, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range header {
		r.Header[k] = v
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func decodeState(t *testing.T, w *httptest.ResponseRecorder) board.Board {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var body board.Board
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	return body
}

func TestStateAnswersEveryRepoWithItsFlags(t *testing.T) {
	code := homeWith(t, map[string]string{"house-move": houseMove, "work": work})
	h := Handler(Options{})

	body := decodeState(t, get(t, h, "/api/state", nil))
	if len(body.Repos) != 2 {
		t.Fatalf("repos = %+v, want two", body.Repos)
	}
	house, report := body.Repos[0], body.Repos[1]
	if house.Name != "house-move" || house.Path != filepath.Join(code, "house-move") || house.Color != "green" {
		t.Errorf("house-move = %+v", house)
	}
	if len(house.Tasks) != 3 || !house.Tasks[2].Blocked || house.Tasks[1].Parents[0] != "Pack the kitchen" {
		t.Errorf("house-move's tasks = %+v", house.Tasks)
	}
	// A clean Repo says so with an empty list; one with a problem is flagged
	// by it, line and all.
	if house.Problems == nil || len(house.Problems) != 0 {
		t.Errorf("house-move problems = %#v, want []", house.Problems)
	}
	if len(report.Problems) != 1 || report.Problems[0].Line != 3 || !strings.Contains(report.Problems[0].Message, "no id") {
		t.Errorf("work problems = %+v, want the missing id on line 3", report.Problems)
	}
	if body.Errors == nil {
		t.Error("errors = null, want []")
	}

	// Rescanned per request: a Repo made after the handler was built is read.
	place(t, filepath.Join(code, "garden"), "# Tasks\n")
	if again := decodeState(t, get(t, h, "/api/state", nil)); len(again.Repos) != 3 {
		t.Errorf("repos after adding one = %d, want 3", len(again.Repos))
	}
}

func TestStateNarrowsTheWayTasksListDoes(t *testing.T) {
	homeWith(t, map[string]string{"house-move": houseMove, "work": work})
	h := Handler(Options{})
	cases := map[string]int{
		"?repo=work":                   1,
		"?state=doing":                 2,
		"?state=backlog&state=inbox":   2,
		"?tag=kitchen":                 1,
		"?search=VAN":                  1,
		"?unblocked=true":              3,
		"?repo=house-move&state=inbox": 0,
		"?repo=&tag=&search=&state=":   -1, // an empty parameter narrows nothing
		"?repo=house-move&unblocked=1": 2,
	}
	for query, want := range cases {
		body := decodeState(t, get(t, h, "/api/state"+query, nil))
		n := 0
		for _, r := range body.Repos {
			n += len(r.Tasks)
		}
		if want == -1 {
			want = 4
		}
		if n != want {
			t.Errorf("%s: %d Tasks, want %d", query, n, want)
		}
	}
}

func TestStateRefusesWhatItCannotAnswer(t *testing.T) {
	homeWith(t, map[string]string{"work": work})
	h := Handler(Options{})
	for query, status := range map[string]int{
		"?repo=nowhere":    http.StatusNotFound,
		"?state=someday":   http.StatusBadRequest,
		"?unblocked=maybe": http.StatusBadRequest,
	} {
		w := get(t, h, "/api/state"+query, http.Header{"If-None-Match": {"*"}})
		if w.Code != status || !strings.Contains(w.Body.String(), `"error"`) {
			t.Errorf("%s = %d %s, want %d with an error sentence", query, w.Code, w.Body.String(), status)
		}
	}
}

func TestStateETagFollowsTheFilesAndTheQuery(t *testing.T) {
	code := homeWith(t, map[string]string{"work": work})
	h := Handler(Options{})

	first := get(t, h, "/api/state", nil)
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag")
	}
	for _, header := range []string{tag, "W/" + tag, `"other", ` + tag, "*"} {
		if w := get(t, h, "/api/state", http.Header{"If-None-Match": {header}}); w.Code != http.StatusNotModified {
			t.Errorf("If-None-Match %s = %d, want 304", header, w.Code)
		}
	}
	if w := get(t, h, "/api/state?state=inbox", http.Header{"If-None-Match": {tag}}); w.Code != http.StatusOK {
		t.Errorf("another query under the same tag = %d, want 200", w.Code)
	}

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(code, "work", "tasks.md"), later, later); err != nil {
		t.Fatal(err)
	}
	if w := get(t, h, "/api/state", http.Header{"If-None-Match": {tag}}); w.Code != http.StatusOK {
		t.Errorf("after the file changed = %d, want 200", w.Code)
	}
}

// A route this package does not serve is a usage error with a sentence in it,
// even with the client's files being served underneath. The fallback answers
// any path it is given, so an /api/ typo would otherwise be index.html at 200
// and the client would fail parsing HTML as JSON instead of saying what
// happened.
func TestAnAPIRouteThatIsNotOneIsNotTheClient(t *testing.T) {
	homeWith(t, nil)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>tasks</title>"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	served := Handler(Options{Web: dir})

	for _, r := range []struct{ method, target string }{
		{http.MethodGet, "/api/stat"},
		{http.MethodPost, "/api/state"},
		{http.MethodGet, "/api/tasks/nope"},
	} {
		w := httptest.NewRecorder()
		served.ServeHTTP(w, httptest.NewRequest(r.method, r.target, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s %s = %d, want 400 rather than the client", r.method, r.target, w.Code)
		}
		if strings.Contains(w.Body.String(), "<title>") {
			t.Errorf("%s %s was answered with the client's index.html", r.method, r.target)
		}
	}

	// A path the client routes in the browser still reaches the client.
	if w := get(t, served, "/board", nil); w.Code != http.StatusOK {
		t.Errorf("/board = %d, want the client at 200", w.Code)
	}

	// Serving the JSON alone says the same thing about the same bad route.
	alone := get(t, Handler(Options{}), "/api/typo", nil)
	if alone.Code != http.StatusBadRequest || !strings.Contains(alone.Body.String(), "is not a route") {
		t.Errorf("/api/typo with no client served = %d %s, want 400 saying so", alone.Code, alone.Body.String())
	}
}

// A path that climbs out of the served directory reaches the client's
// index.html, never the file it named. http.Dir refuses the name before
// anything is opened, and this test asks client directly because ServeMux
// cleans a path of its own accord and would never hand one like this over.
func TestAPathThatClimbsOutOfTheServedDirectoryDoesNot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "web")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>tasks</title>"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("not for the browser"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	for _, target := range []string{"/../secret", "/..%2fsecret", "/web/../../secret"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://tasks.test", nil)
		r.URL.Path = target
		client(dir).ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "not for the browser") {
			t.Errorf("%s was served the file above the directory", target)
		}
	}
}
