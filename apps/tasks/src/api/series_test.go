package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const errands = `# Tasks

- [ ] Renew the passport | deferred
  - id: pp01
  - created: 2026-09-01
  - until: 2026-10-02
- [ ] Take the bins out | backlog
  - id: bin1
  - created: 2026-09-01
  - deadline: 2026-10-05
  - series: every week on mon from 2026-01-05
`

// day is a moment on the date, in the host's zone.
func day(t *testing.T, date string, hour int) time.Time {
	t.Helper()
	when, err := time.ParseInLocation(time.DateOnly, date, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return when.Add(time.Duration(hour) * time.Hour)
}

func TestSeriesAnswersTheRuleAndItsNextDates(t *testing.T) {
	homeWith(t, map[string]string{"errands": errands})
	h := Handler(Options{Now: func() time.Time { return day(t, "2026-10-01", 9) }})

	w := get(t, h, "/api/tasks/bin1/series?repo=errands", nil)
	want := `{"rule":"every week on mon from 2026-01-05","dates":["2026-10-05","2026-10-12","2026-10-19","2026-10-26","2026-11-02"]}`
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("GET series answered %d %s, want %s", w.Code, w.Body.String(), want)
	}
	w = get(t, h, "/api/tasks/pp01/series?repo=errands", nil)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"rule":"","dates":[]}` {
		t.Errorf("GET series of a Task with no rule answered %d %s", w.Code, w.Body.String())
	}
	for target, status := range map[string]int{
		"/api/tasks/zzzz/series?repo=errands": http.StatusNotFound,
		"/api/tasks/bin1/series?repo=nowhere": http.StatusNotFound,
		"/api/tasks/bin1/series":              http.StatusBadRequest,
	} {
		w := get(t, h, target, nil)
		var body struct{ Error string }
		if w.Code != status || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Error == "" {
			t.Errorf("GET %s answered %d %s, want %d with a sentence", target, w.Code, w.Body.String(), status)
		}
	}
}

func TestAReadRunsThePassFirstAsTheTaskServer(t *testing.T) {
	code := homeWith(t, map[string]string{"errands": errands})
	gitUser(t)
	h := Handler(Options{Actor: "neal", Now: func() time.Time { return day(t, "2026-10-02", 9) }})

	house := decodeState(t, get(t, h, "/api/state", nil)).Repos[0]
	if house.Tasks[0].ID != "pp01" || house.Tasks[0].State != "backlog" {
		t.Errorf("GET /api/state answered %+v, want pp01 woken to backlog", house.Tasks[0])
	}
	if got := lastCommit(t, filepath.Join(code, "errands")); got != "chore(tasks): wake ^pp01 to Backlog|task server" {
		t.Errorf("the last commit is %q, want the wake by the task server", got)
	}
}

// clock is a host clock a test winds on, and the timer `tasks api` waits on
// for midnight, which fires when the test says.
type clock struct {
	mu    sync.Mutex
	now   time.Time
	asked chan time.Duration
	fire  chan time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

func (c *clock) After(d time.Duration) <-chan time.Time {
	c.asked <- d
	return c.fire
}

func TestThePassRunsJustAfterLocalMidnight(t *testing.T) {
	code := homeWith(t, map[string]string{"errands": errands})
	gitUser(t)
	c := &clock{now: day(t, "2026-10-01", 23), asked: make(chan time.Duration), fire: make(chan time.Time)}
	o := Options{Now: c.Now, After: c.After}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		nightly(o, stop)
		close(done)
	}()

	if d := <-c.asked; d != time.Hour+time.Second {
		t.Errorf("at 23:00 the pass waits %v, want until one second past midnight", d)
	}
	before, err := os.ReadFile(filepath.Join(code, "errands", "TASKS.md"))
	if err != nil || string(before) != errands {
		t.Fatalf("the pass ran before midnight: %s", before)
	}
	c.set(day(t, "2026-10-02", 0).Add(time.Second))
	c.fire <- c.Now()
	// It asks for the next midnight once this one's pass is written.
	if d := <-c.asked; d != 24*time.Hour {
		t.Errorf("after the pass it waits %v, want a day", d)
	}
	if got := lastCommit(t, filepath.Join(code, "errands")); got != "chore(tasks): wake ^pp01 to Backlog|task server" {
		t.Errorf("the last commit is %q, want the midnight wake by the task server", got)
	}
	close(stop)
	<-done
}

func lastCommit(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "--git-dir=.tasks.git",
		"log", "-1", "--format=%s|%(trailers:key=Generated-By,valueonly,separator=)").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
