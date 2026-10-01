package api

import (
	"testing"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
)

const packed = `# Tasks

- [x] Pack the kitchen | done #kitchen
  - id: pk01
  - created: 2026-09-14
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
- [-] Book the van | declined #van
  - id: vn01
  - created: 2026-09-29
  - ended: 2026-09-30
`

// counted is the weeks that hold anything, by their Monday.
func counted(weeks []board.Week) map[string]board.Week {
	out := map[string]board.Week{}
	for _, w := range weeks {
		if w.Added+w.Done+w.Declined > 0 {
			out[w.Start] = w
		}
	}
	return out
}

func TestStateCarriesTwelveWeeksOfMetricsUnderTheChips(t *testing.T) {
	homeWith(t, map[string]string{"house-move": packed, "work": work})
	h := Handler(Options{Now: func() time.Time { return day(t, "2026-10-01", 9) }})

	wide := decodeState(t, get(t, h, "/api/state", nil)).Metrics
	if len(wide) != 12 || wide[0].Start != "2026-07-13" || wide[11].Start != "2026-09-28" {
		t.Fatalf("metrics = %+v, want twelve weeks from 2026-07-13 to 2026-09-28", wide)
	}
	want := map[string]board.Week{
		"2026-09-14": {Start: "2026-09-14", Added: 4},
		"2026-09-21": {Start: "2026-09-21", Added: 1, Done: 4}, // the report in work
		"2026-09-28": {Start: "2026-09-28", Added: 1, Declined: 1},
	}
	if got := counted(wide); !equal(got, want) {
		t.Errorf("wide metrics = %+v, want %+v", got, want)
	}

	// The chips narrow the metrics; the search, the States and the sort do not.
	tagged := decodeState(t, get(t, h, "/api/state?repo=house-move&tag=van&search=nothing&state=inbox&sort=title", nil)).Metrics
	want = map[string]board.Week{"2026-09-28": {Start: "2026-09-28", Added: 1, Declined: 1}}
	if got := counted(tagged); !equal(got, want) {
		t.Errorf("metrics under repo and tag = %+v, want %+v", got, want)
	}
	work := decodeState(t, get(t, h, "/api/state?repo=work", nil)).Metrics
	if got := counted(work); len(got) != 1 || got["2026-09-21"].Added != 1 {
		t.Errorf("metrics under repo=work = %+v, want the report added the week of 2026-09-21", got)
	}
}

func equal(a, b map[string]board.Week) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
