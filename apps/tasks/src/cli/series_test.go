package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// at sets the host's clock for the rest of the test, so "today" is a date the
// test names rather than whatever day it runs on.
func at(t *testing.T, date string) {
	t.Helper()
	when, err := time.ParseInLocation(time.DateOnly, date, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	was := now
	now = func() time.Time { return when.Add(9 * time.Hour) }
	t.Cleanup(func() { now = was })
}

// listing runs `tasks list`, which reads and so runs the pass first.
func listing(t *testing.T) {
	t.Helper()
	if status, _, errs := run(t, "list"); status != 0 {
		t.Fatalf("tasks list exited %d: %s", status, errs)
	}
}

func authored(t *testing.T, repo string) string {
	t.Helper()
	return strings.Join(tasksLog(t, repo, "%s|%(trailers:key=Generated-By,valueonly,separator=)"), "\n")
}

const weekly = `# Tasks

- [ ] Take the bins out | backlog #home
  - id: bin1
  - created: 2026-09-01
  - deadline: 2026-10-12
  - series: every week on mon from 2026-01-05
  - priority: high

  Both of them.

  - [ ] Green bin | backlog
    - id: bin2
    - created: 2026-09-01
  - [ ] Black bin | backlog
    - id: bin3
    - created: 2026-09-01
    - blocked by: bin2
`

func TestRepeatSetsARuleShowsItsNextDatesAndTurnsItOff(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	at(t, "2026-10-01")

	status, out, errs := run(t, "repeat", "v9t1", "every", "2", "weeks", "on", "tue", "from", "2026-09-01", "-repo", "house-move")
	if status != 0 {
		t.Fatalf("tasks repeat exited %d: %s", status, errs)
	}
	want := "every 2 weeks on tue from 2026-09-01\n2026-10-13\n2026-10-27\n2026-11-10\n2026-11-24\n2026-12-08\n"
	if out != want {
		t.Errorf("tasks repeat printed\n%s\nwant\n%s", out, want)
	}
	if file := read(t, filepath.Join(repo, "TASKS.md")); !strings.Contains(file, "  - blocked by: m3qa\n  - series: every 2 weeks on tue from 2026-09-01\n") {
		t.Errorf("TASKS.md =\n%s\nwant v9t1 to carry the rule", file)
	}

	// Asked with no rule, it shows the one there is and writes nothing.
	if status, shown, _ := run(t, "repeat", "v9t1", "-repo", "house-move"); status != 0 || shown != want {
		t.Errorf("tasks repeat v9t1 exited %d and printed\n%s\nwant\n%s", status, shown, want)
	}

	if status, _, errs := run(t, "repeat", "v9t1", "-off", "-repo", "house-move"); status != 0 {
		t.Fatalf("tasks repeat -off exited %d: %s", status, errs)
	}
	if file := read(t, filepath.Join(repo, "TASKS.md")); strings.Contains(file, "series") {
		t.Errorf("after -off, TASKS.md =\n%s\nwant no rule", file)
	}
	subjects := tasksLog(t, repo, "%s")
	wantSubjects := []string{
		"chore(tasks): repeat ^v9t1 off",
		"chore(tasks): repeat ^v9t1 every 2 weeks on tue from 2026-09-01",
		"chore(tasks): commit a direct edit",
	}
	if strings.Join(subjects, "\n") != strings.Join(wantSubjects, "\n") {
		t.Errorf("tasks history =\n%s\nwant\n%s", strings.Join(subjects, "\n"), strings.Join(wantSubjects, "\n"))
	}

	if status, out, _ := run(t, "repeat", "v9t1", "-repo", "house-move"); status != 0 || out != "v9t1 does not repeat\n" {
		t.Errorf("tasks repeat on a Task with no rule exited %d and printed %q", status, out)
	}
	for _, c := range []struct {
		args   []string
		status int
	}{
		{[]string{"repeat", "v9t1", "every", "fortnight", "-repo", "house-move"}, 2},
		{[]string{"repeat", "v9t1", "-off", "every", "week", "-repo", "house-move"}, 2},
		{[]string{"repeat", "-repo", "house-move"}, 2},
		{[]string{"repeat", "zzzz", "every", "week", "-repo", "house-move"}, 1},
	} {
		if status, _, errs := run(t, c.args...); status != c.status || errs == "" {
			t.Errorf("tasks %v exited %d (%q), want %d with a sentence", c.args, status, errs, c.status)
		}
	}
}

func TestEndingAnOccurrenceWritesTheNextOnTheNextPass(t *testing.T) {
	code := gitHome(t, map[string]string{"home": weekly})
	repo := filepath.Join(code, "home")
	withRemote(t, repo)
	at(t, "2026-10-01")

	moving(t, "bin2", "done", "-repo", "home")
	moving(t, "bin3", "declined", "-repo", "home")
	file := read(t, filepath.Join(repo, "TASKS.md"))
	if strings.Count(file, "series:") != 1 {
		t.Fatalf("the move wrote the next Occurrence itself:\n%s", file)
	}

	listing(t)
	file = read(t, filepath.Join(repo, "TASKS.md"))
	next := regexpID(t, file, "Take the bins out | backlog #home")
	green := regexpID(t, file, "Green bin | backlog")
	ended := today()
	// An early finish: the ended one was due 2026-10-12, so the next is the
	// Monday after that, never the 12th again.
	want := `# Tasks

- [ ] Take the bins out | backlog #home
  - id: ` + next + `
  - created: 2026-10-01
  - deadline: 2026-10-19
  - series: every week on mon from 2026-01-05
  - priority: high

  Both of them.

  - [ ] Green bin | backlog
    - id: ` + green + `
    - created: 2026-10-01
  - [ ] Black bin | backlog
    - id: ` + regexpID(t, file, "Black bin | backlog") + `
    - created: 2026-10-01
    - blocked by: ` + green + `
- [x] Take the bins out | done #home
  - id: bin1
  - created: 2026-09-01
  - deadline: 2026-10-12
  - priority: high
  - ended: ` + ended + `

  Both of them.

  - [x] Green bin | done
    - id: bin2
    - created: 2026-09-01
    - ended: ` + ended + `
  - [-] Black bin | declined
    - id: bin3
    - created: 2026-09-01
    - blocked by: bin2
    - ended: ` + ended + `
`
	if file != want {
		t.Errorf("after the pass, TASKS.md =\n%s\nwant\n%s", file, want)
	}

	// The rule moved off the ended one, so the next pass writes nothing.
	listing(t)
	got := authored(t, repo)
	wantLog := strings.Join([]string{
		"chore(tasks): next ^" + next + " after ^bin1|task server",
		"chore(tasks): move ^bin3 to Declined|claude-code/claude-opus-5-5",
		"chore(tasks): move ^bin2 to Done|claude-code/claude-opus-5-5",
		"chore(tasks): commit a direct edit|direct edit",
	}, "\n")
	if got != wantLog {
		t.Errorf("tasks history =\n%s\nwant\n%s", got, wantLog)
	}
}

func TestAHandTickedOccurrenceLongOverdueRepeatsOnceFromToday(t *testing.T) {
	code := gitHome(t, map[string]string{"home": `# Tasks

- [x] Water the plants | backlog
  - id: wat1
  - created: 2026-01-01
  - deadline: 2026-08-03
  - series: every week on mon from 2026-01-05
`})
	repo := filepath.Join(code, "home")
	at(t, "2026-10-01")

	listing(t)
	listing(t)
	file := read(t, filepath.Join(repo, "TASKS.md"))
	// Every Monday since August was missed; one Occurrence comes back, due on
	// the first Monday after today.
	if strings.Count(file, "Water the plants") != 2 || !strings.Contains(file, "- deadline: 2026-10-05\n  - series: every week on mon from 2026-01-05\n") {
		t.Errorf("after two passes, TASKS.md =\n%s\nwant one next Occurrence due 2026-10-05", file)
	}
	if !strings.Contains(file, "- [x] Water the plants | done\n  - id: wat1\n  - created: 2026-01-01\n  - deadline: 2026-08-03\n") {
		t.Errorf("TASKS.md =\n%s\nwant the hand-ticked one Done with its rule moved off", file)
	}
	if got := authored(t, repo); !strings.HasPrefix(got, "chore(tasks): next ^") || !strings.HasSuffix(got, "after ^wat1|task server\nchore(tasks): commit a direct edit|direct edit") {
		t.Errorf("tasks history =\n%s\nwant the hand edit and then one next Occurrence by the task server", got)
	}
}

func TestARuleThatHasRunOutEndsTheSeries(t *testing.T) {
	code := gitHome(t, map[string]string{"home": `# Tasks

- [x] File the tax return | done
  - id: tax1
  - created: 2026-01-01
  - series: every year from 2025-04-15 until 2026-04-15
`})
	repo := filepath.Join(code, "home")
	at(t, "2026-10-01")

	listing(t)
	if file := read(t, filepath.Join(repo, "TASKS.md")); strings.Contains(file, "series") || strings.Count(file, "File the tax return") != 1 {
		t.Errorf("TASKS.md =\n%s\nwant the rule gone and no next Occurrence", file)
	}
	if got := authored(t, repo); !strings.HasPrefix(got, "chore(tasks): end the series of ^tax1|task server") {
		t.Errorf("tasks history =\n%s", got)
	}
}

func TestADeferredTaskWakesToBacklogOnItsUntilDate(t *testing.T) {
	code := gitHome(t, map[string]string{"errands": `# Tasks

- [ ] Renew the passport | deferred
  - id: pp01
  - created: 2026-09-01
  - until: 2026-10-01
  - reason: waiting on photos
- [ ] Paint the fence | deferred
  - id: pf01
  - created: 2026-09-01
  - until: 2026-10-02
- [ ] Learn the cello | deferred
  - id: lc01
  - created: 2026-09-01
`})
	repo := filepath.Join(code, "errands")
	at(t, "2026-10-01")

	// Any read runs the pass, and so does a write before it writes.
	if status, out, errs := run(t, "list", "-state", "backlog"); status != 0 || !strings.Contains(out, "pp01 backlog Renew the passport") {
		t.Fatalf("tasks list exited %d, printed\n%s%s\nwant pp01 woken into the read", status, out, errs)
	}
	want := `# Tasks

- [ ] Renew the passport | backlog
  - id: pp01
  - created: 2026-09-01
- [ ] Paint the fence | deferred
  - id: pf01
  - created: 2026-09-01
  - until: 2026-10-02
- [ ] Learn the cello | deferred
  - id: lc01
  - created: 2026-09-01
`
	if file := read(t, filepath.Join(repo, "TASKS.md")); file != want {
		t.Errorf("TASKS.md =\n%s\nwant\n%s", file, want)
	}

	at(t, "2026-10-03")
	if status, _, errs := run(t, "add", "Buy stamps", "-repo", "errands"); status != 0 {
		t.Fatalf("tasks add exited %d: %s", status, errs)
	}
	got := tasksLog(t, repo, "%s|%(trailers:key=Generated-By,valueonly,separator=)")
	if len(got) != 4 || !strings.HasPrefix(got[0], "chore(tasks): add ^") ||
		got[1] != "chore(tasks): wake ^pf01 to Backlog|task server" ||
		got[2] != "chore(tasks): wake ^pp01 to Backlog|task server" {
		t.Errorf("tasks history =\n%s\nwant each wake by the task server, then the add", strings.Join(got, "\n"))
	}
}

func TestARepoRefusingWritesIsSkippedAndTheCallGoesAhead(t *testing.T) {
	code := gitHome(t, map[string]string{
		"broken": "# Tasks\n\n<<<<<<< ours\n- [ ] Mine | deferred\n  - id: m1n3\n  - created: 2026-09-01\n  - until: 2026-09-02\n=======\n>>>>>>> theirs\n",
		"errands": `# Tasks

- [ ] Renew the passport | deferred
  - id: pp01
  - created: 2026-09-01
  - until: 2026-09-30
`,
	})
	at(t, "2026-10-01")
	before := read(t, filepath.Join(code, "broken", "TASKS.md"))

	status, out, errs := run(t, "list")
	if status != 0 || !strings.Contains(out, "pp01 backlog") {
		t.Errorf("tasks list exited %d and printed\n%s%s\nwant the other Repo woken and listed", status, out, errs)
	}
	if after := read(t, filepath.Join(code, "broken", "TASKS.md")); after != before {
		t.Errorf("the pass wrote over a Repo refusing writes:\n%s", after)
	}
	if flags := flagsOf(t, "broken"); !strings.Contains(flags, "conflict markers") {
		t.Errorf("broken is flagged %q, want it refusing writes", flags)
	}
}

// regexpID is the id under the first title line holding the text.
func regexpID(t *testing.T, file, title string) string {
	t.Helper()
	_, after, ok := strings.Cut(file, title+"\n")
	if !ok {
		t.Fatalf("no %q in\n%s", title, file)
	}
	line, _, _ := strings.Cut(strings.TrimSpace(after), "\n")
	id, ok := strings.CutPrefix(line, "- id: ")
	if !ok || !anID.MatchString(id) {
		t.Fatalf("no id under %q in\n%s", title, file)
	}
	return id
}
