package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
)

// versionOf is the version `tasks list -json` gives the Task's tree, which is
// what a caller that showed the Task hands back to `tasks edit`.
func versionOf(t *testing.T, repo, id string) string {
	t.Helper()
	status, out, errs := run(t, "list", "-json", "-repo", repo)
	if status != 0 {
		t.Fatalf("tasks list -json exited %d: %s", status, errs)
	}
	var b board.Board
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatal(err)
	}
	for _, task := range b.Repos[0].Tasks {
		if task.ID == id {
			if task.Version == "" {
				t.Fatalf("^%s was read with no version", id)
			}
			return task.Version
		}
	}
	t.Fatalf("tasks list -json holds no ^%s", id)
	return ""
}

func editing(t *testing.T, args ...string) {
	t.Helper()
	if status, _, errs := run(t, append([]string{"edit"}, args...)...); status != 0 {
		t.Fatalf("tasks edit %v exited %d: %s", args, status, errs)
	}
}

const errands = `# Tasks

- [ ] Renew the passport | deferred #admin
  - id: pp01
  - created: 2026-09-01
  - until: 2099-12-01
  - reason: waiting on photos
  - mood: grim
  - attachment: ~/scans/old.pdf
- [ ] Buy stamps | inbox
  - id: st01
  - created: 2026-09-02
`

func TestEditSetsEveryAttributeAndKeepsUnknownLabels(t *testing.T) {
	code := gitHome(t, map[string]string{"errands": errands})
	repo := filepath.Join(code, "errands")
	withRemote(t, repo)

	editing(t, "pp01", "-repo", "errands",
		"-title", "Renew both passports",
		"-tag", "admin", "-tag", "travel",
		"-description", "Both expire in spring.",
		"-why", "the trip in May",
		"-acceptance", "both in the drawer",
		"-deadline", "2027-03-01",
		"-priority", "high",
		"-impact", "med",
		"-estimate", "small",
		"-color", "blue",
		"-blocked-by", "st01",
		"-reason", "waiting on new photos",
		"-until", "2026-11-15",
		"-attach", "~/scans/new.pdf",
		"-detach", "~/scans/old.pdf",
	)
	want := `# Tasks

- [ ] Renew both passports | deferred #admin #travel
  - id: pp01
  - created: 2026-09-01
  - until: 2026-11-15
  - reason: waiting on new photos
  - mood: grim
  - deadline: 2027-03-01
  - priority: high
  - impact: med
  - estimate: small
  - color: blue
  - why: the trip in May
  - acceptance: both in the drawer
  - blocked by: st01
  - attachment: ~/scans/new.pdf

  Both expire in spring.
- [ ] Buy stamps | inbox
  - id: st01
  - created: 2026-09-02
`
	if got := read(t, filepath.Join(repo, "tasks.md")); got != want {
		t.Errorf("after the edit, tasks.md =\n%s\nwant\n%s", got, want)
	}
	if subjects := tasksLog(t, repo, "%s"); subjects[0] != "chore(tasks): edit ^pp01 Renew both passports" {
		t.Errorf("the edit committed as %q", subjects[0])
	}

	// An empty value takes the attribute off; one not given is left alone.
	editing(t, "pp01", "-repo", "errands", "-tag", "", "-why", "", "-blocked-by", "", "-description", "")
	got := read(t, filepath.Join(repo, "tasks.md"))
	if !strings.HasPrefix(got, "# Tasks\n\n- [ ] Renew both passports | deferred\n") ||
		strings.Contains(got, "why:") || strings.Contains(got, "blocked by:") || strings.Contains(got, "Both expire") ||
		!strings.Contains(got, "  - mood: grim\n") || !strings.Contains(got, "  - priority: high\n") {
		t.Errorf("after clearing, tasks.md =\n%s", got)
	}
}

func TestEditsTheRulesTurnAwayChangeNothing(t *testing.T) {
	code := gitHome(t, map[string]string{"errands": errands})
	repo := filepath.Join(code, "errands")
	withRemote(t, repo)
	before := read(t, filepath.Join(repo, "tasks.md"))

	for _, c := range []struct {
		args   []string
		status int
		says   string
	}{
		{[]string{"st01", "-reason", "no"}, 2, "Deferred or Declined"},
		{[]string{"st01", "-until", "2026-12-01"}, 2, "only while a Task is Deferred"},
		{[]string{"pp01", "-until", "soon"}, 2, "not a date"},
		{[]string{"pp01", "-priority", "urgent"}, 2, "not one of"},
		{[]string{"pp01", "-blocked-by", "zzzz"}, 2, "no Task in this Repo"},
		{[]string{"pp01", "-blocked-by", "pp01"}, 2, "cannot wait on itself"},
		{[]string{"pp01", "-title", ""}, 2, "title of one line"},
		{[]string{"zzzz", "-title", "x"}, 1, "no Task has the id zzzz"},
		{[]string{"pp01", "-version", "stale"}, 3, "changed since"},
		{[]string{}, 2, "want an id"},
	} {
		status, _, errs := run(t, append(append([]string{"edit"}, c.args...), "-repo", "errands")...)
		if status != c.status || !strings.Contains(errs, c.says) {
			t.Errorf("tasks edit %v exited %d saying %q, want %d saying %q", c.args, status, errs, c.status, c.says)
		}
	}
	if got := read(t, filepath.Join(repo, "tasks.md")); got != before {
		t.Errorf("a refused edit changed tasks.md to\n%s", got)
	}
}

func TestAStaleTreeIsRefusedWhileAnotherTreeGoesThrough(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)

	// Two callers read the board. One edits a Subtask of the kitchen, which
	// changes the kitchen's whole tree.
	kitchen, van := versionOf(t, "house-move", "m3qc"), versionOf(t, "house-move", "v9t1")
	if kitchen != versionOf(t, "house-move", "m3qa") {
		t.Fatalf("a Subtask's version is not its tree's")
	}
	editing(t, "m3qc", "-repo", "house-move", "-version", kitchen, "-priority", "high")

	// The other, still holding what it read, edits the kitchen's parent: stale.
	status, _, errs := run(t, "edit", "m3qa", "-repo", "house-move", "-version", kitchen, "-title", "Pack it all")
	if status != 3 || !strings.Contains(errs, "changed since") {
		t.Errorf("a stale edit exited %d saying %q, want 3", status, errs)
	}
	status, _, errs = run(t, "delete", "m3qb", "-repo", "house-move", "-version", kitchen)
	if status != 3 || !strings.Contains(errs, "changed since") {
		t.Errorf("a stale delete exited %d saying %q, want 3", status, errs)
	}
	// The van's tree did not change, so its edit goes through.
	editing(t, "v9t1", "-repo", "house-move", "-version", van, "-title", "Book the big van")

	if got := read(t, filepath.Join(repo, "tasks.md")); strings.Contains(got, "Pack it all") || !strings.Contains(got, "Book the big van") || !strings.Contains(got, "Buy boxes") {
		t.Errorf("tasks.md =\n%s", got)
	}
}

func TestDeleteRemovesTheBlockAndItsSubtasks(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)

	if status, _, errs := run(t, "delete", "m3qa", "-repo", "house-move"); status != 0 {
		t.Fatalf("tasks delete exited %d: %s", status, errs)
	}
	// The van's blocker went with it, and a blocker naming nothing blocks
	// nothing, so the line goes too rather than leave a lint problem.
	want := "# Tasks\ncolor: green\n\n- [ ] Book the van | backlog\n  - id: v9t1\n  - created: 2026-09-21\n"
	if got := read(t, filepath.Join(repo, "tasks.md")); got != want {
		t.Errorf("after the delete, tasks.md =\n%s\nwant\n%s", got, want)
	}
	if subjects := tasksLog(t, repo, "%s"); subjects[0] != "chore(tasks): delete ^m3qa Pack the kitchen" {
		t.Errorf("the delete committed as %q", subjects[0])
	}
	// The history keeps it.
	if old := git(t, repo, "--git-dir=.tasks.git", "show", "HEAD~1:tasks.md"); !strings.Contains(old, "Wrap glassware") {
		t.Errorf("the history lost the deleted block:\n%s", old)
	}

	// Deleting the last open Subtask settles its parent.
	place(t, repo, houseMove)
	if status, _, errs := run(t, "delete", "m3qc", "-repo", "house-move"); status != 0 {
		t.Fatalf("tasks delete exited %d: %s", status, errs)
	}
	if got := read(t, filepath.Join(repo, "tasks.md")); !strings.Contains(got, "- [x] Pack the kitchen | done #packing #kitchen\n") {
		t.Errorf("the parent did not settle:\n%s", got)
	}

	if status, _, errs := run(t, "delete", "zzzz", "-repo", "house-move"); status != 1 || !strings.Contains(errs, "no Task has the id zzzz") {
		t.Errorf("deleting what is not there exited %d saying %q", status, errs)
	}
}

func TestAHandTypedTaskGetsItsIDAndCreatedOnTheNextWrite(t *testing.T) {
	code := gitHome(t, map[string]string{"errands": "# Tasks\n\n- [ ] Call the bank | inbox\n- [ ] Buy stamps | inbox\n  - id: st01\n  - created: 2026-09-02\n"})
	repo := filepath.Join(code, "errands")
	withRemote(t, repo)

	editing(t, "st01", "-repo", "errands", "-priority", "low")
	got := read(t, filepath.Join(repo, "tasks.md"))
	lines := strings.Split(got, "\n")
	if lines[2] != "- [ ] Call the bank | inbox" || !anID.MatchString(strings.TrimPrefix(lines[3], "  - id: ")) || lines[4] != "  - created: "+today() {
		t.Errorf("the hand-typed Task was not given its id and created:\n%s", got)
	}
}

func TestTagsCountsAcrossEveryRepoAndRenameIsOneCommitEach(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove, "errands": errands, "work": work})
	for _, name := range []string{"house-move", "errands", "work"} {
		withRemote(t, filepath.Join(code, name))
	}

	status, out, errs := run(t, "tags")
	if status != 0 {
		t.Fatalf("tasks tags exited %d: %s", status, errs)
	}
	if fields := strings.Fields(out); strings.Join(fields, " ") != "admin 1 kitchen 2 packing 1 writing 1" {
		t.Errorf("tasks tags printed\n%s", out)
	}

	if status, _, errs := run(t, "tags", "rename", "kitchen", "admin"); status != 0 {
		t.Fatalf("tasks tags rename exited %d: %s", status, errs)
	}
	house := read(t, filepath.Join(code, "house-move", "tasks.md"))
	if !strings.Contains(house, "| doing #packing #admin\n") || !strings.Contains(house, "| doing #admin\n") || strings.Contains(house, "#kitchen") {
		t.Errorf("house-move after the rename:\n%s", house)
	}
	if subjects := tasksLog(t, filepath.Join(code, "house-move"), "%s"); subjects[0] != "chore(tasks): rename #kitchen to #admin" || len(subjects) != 2 {
		t.Errorf("house-move's history = %q, want one rename commit after the direct edit", subjects)
	}
	// A Repo carrying no #kitchen is not written to at all.
	for _, name := range []string{"errands", "work"} {
		if _, err := os.Stat(filepath.Join(code, name, ".tasks.git")); err == nil {
			t.Errorf("%s was written to by a rename it had no part in", name)
		}
	}

	// Renaming to a Tag the Task already carries leaves one of it.
	if status, _, errs := run(t, "tags", "rename", "packing", "admin"); status != 0 {
		t.Fatalf("tasks tags rename exited %d: %s", status, errs)
	}
	if house := read(t, filepath.Join(code, "house-move", "tasks.md")); !strings.Contains(house, "| doing #admin\n  - id: m3qa") {
		t.Errorf("house-move after merging Tags:\n%s", house)
	}

	// Two Repos carry #admin now: one commit in each.
	if status, _, errs := run(t, "tags", "rename", "admin", "paperwork"); status != 0 {
		t.Fatalf("tasks tags rename exited %d: %s", status, errs)
	}
	for _, name := range []string{"house-move", "errands"} {
		if subjects := tasksLog(t, filepath.Join(code, name), "%s"); subjects[0] != "chore(tasks): rename #admin to #paperwork" ||
			(len(subjects) > 1 && strings.Contains(subjects[1], "#paperwork")) {
			t.Errorf("%s's history = %q, want one rename commit", name, subjects)
		}
	}

	if status, _, errs := run(t, "tags", "rename", "admin", "two words"); status != 2 || !strings.Contains(errs, "not a Tag") {
		t.Errorf("renaming to a bad Tag exited %d saying %q", status, errs)
	}
}
