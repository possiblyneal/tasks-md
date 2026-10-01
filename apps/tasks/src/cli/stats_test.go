package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// packed is a parent whose three Subtasks ended in the week of 2026-09-21,
// the last on the Sunday. The parent's own `ended:` line says otherwise, and
// the day its last Subtask ended is the one that counts.
const packed = `# Tasks

- [x] Pack the kitchen | done #kitchen
  - id: pk01
  - created: 2026-09-14
  - ended: 2026-09-21
  - [x] Buy boxes | done
    - id: pk02
    - created: 2026-09-14
    - ended: 2026-09-21
  - [x] Wrap glassware | done
    - id: pk03
    - created: 2026-09-15
    - ended: 2026-09-23
  - [x] Label the boxes | done
    - id: pk04
    - created: 2026-09-20
    - ended: 2026-09-27
- [ ] Sell the sofa | backlog
  - id: sf01
  - created: 2026-09-30
- [-] Book the van | declined #van
  - id: vn01
  - created: 2026-07-01
  - ended: 2026-09-28
`

const office = `# Tasks

- [ ] Write the report | inbox #kitchen
  - id: r3pt
  - created: 2026-09-29
`

// weeks is the twelve rows `tasks stats` prints, oldest first, with the
// rows named in rows filled in and every other week empty.
func weeks(rows map[string]string) string {
	starts := []string{
		"2026-07-13", "2026-07-20", "2026-07-27", "2026-08-03", "2026-08-10", "2026-08-17",
		"2026-08-24", "2026-08-31", "2026-09-07", "2026-09-14", "2026-09-21", "2026-09-28",
	}
	out := "WEEK        ADDED  DONE  DECLINED\n"
	for _, start := range starts {
		row, ok := rows[start]
		if !ok {
			row = "0      0     0"
		}
		out += start + "  " + row + "\n"
	}
	return out
}

func stated(t *testing.T, args ...string) string {
	t.Helper()
	status, out, errs := run(t, append([]string{"stats"}, args...)...)
	if status != 0 {
		t.Fatalf("tasks stats %v exited %d: %s", args, status, errs)
	}
	return out
}

func TestStatsCountsEveryTaskWeeklyWithAParentEndingOnItsLastSubtasksDay(t *testing.T) {
	home(t, map[string]string{"house-move": packed, "work": office})
	at(t, "2026-10-01")

	want := `LANE      CARDS
inbox     1
backlog   1
doing     0
deferred  0
done      3
declined  1

` + weeks(map[string]string{
		"2026-09-14": "4      0     0",
		"2026-09-21": "0      4     0",
		"2026-09-28": "2      0     1",
	})
	if got := stated(t); got != want {
		t.Errorf("tasks stats =\n%s\nwant\n%s", got, want)
	}
}

func TestStatsNarrowsByRepoAndTag(t *testing.T) {
	home(t, map[string]string{"house-move": packed, "work": office})
	at(t, "2026-10-01")

	// The parent carries the Tag and its Subtasks do not, so it is the one
	// Task counted, and no card is in a lane: a parent is never one.
	got := stated(t, "-tag", "kitchen", "-repo", "house-move")
	want := `LANE      CARDS
inbox     0
backlog   0
doing     0
deferred  0
done      0
declined  0

` + weeks(map[string]string{
		"2026-09-14": "1      0     0",
		"2026-09-21": "0      1     0",
	})
	if got != want {
		t.Errorf("tasks stats -tag kitchen -repo house-move =\n%s\nwant\n%s", got, want)
	}

	got = stated(t, "-tag", "kitchen", "-tag", "van")
	if !strings.Contains(got, "2026-09-28  1      0     1\n") {
		t.Errorf("tasks stats -tag kitchen -tag van counted this week as\n%s\nwant 1 added from work and 1 declined", got)
	}

	if status, _, errs := run(t, "stats", "-repo", "nowhere"); status != 1 || !strings.Contains(errs, "no Repo is named nowhere") {
		t.Errorf("tasks stats -repo nowhere = %d %q, want 1 naming the Repo", status, errs)
	}
	if status, _, _ := run(t, "stats", "extra"); status != 2 {
		t.Errorf("tasks stats extra = %d, want 2", status)
	}
}

func TestADeletedTaskIsInNoWeek(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": packed})
	at(t, "2026-10-01")
	if !strings.Contains(stated(t), "2026-09-28  1      0     1\n") {
		t.Fatal("the sofa was not counted before it was deleted")
	}

	if status, _, errs := run(t, "delete", "sf01", "-repo", "house-move"); status != 0 {
		t.Fatalf("tasks delete exited %d: %s", status, errs)
	}
	if strings.Contains(read(t, filepath.Join(code, "house-move", "TASKS.md")), "sf01") {
		t.Fatal("the sofa is still in the file")
	}
	if got := stated(t); !strings.Contains(got, "2026-09-28  0      0     1\n") {
		t.Errorf("after the delete, tasks stats =\n%s\nwant the sofa in no week", got)
	}
}
