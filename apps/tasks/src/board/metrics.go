package board

import (
	"os"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/repos"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// Week is one week of the metrics: how many Tasks were created in it, and how
// many ended Done and ended Declined. Start is its Monday, host-local.
type Week struct {
	Start    string `json:"start"`
	Added    int    `json:"added"`
	Done     int    `json:"done"`
	Declined int    `json:"declined"`
}

// weeks is how many a read counts back, the one holding today included.
const weeks = 12

// Weekly counts the last twelve weeks, oldest first, from each file's
// `created:` and `ended:` lines as they stand, so a deleted Task is in no
// week. Every Task counts, parents included, and a parent ends on the day the
// last of its Subtasks did. It follows the Narrowing's Repo and Tags alone,
// the way the board's chips narrow, and keeps a Task by its Tags exactly as a
// read does.
func Weekly(found []repos.Repo, n Narrowing, today time.Time) ([]Week, error) {
	found, err := Only(found, n.Repo)
	if err != nil {
		return nil, err
	}
	tagged := Narrowing{Tags: n.Tags}
	y, m, d := today.Date()
	monday := time.Date(y, m, d, 0, 0, 0, 0, today.Location())
	monday = monday.AddDate(0, 0, -(int(monday.Weekday())+6)%7)
	out := make([]Week, weeks)
	for i := range out {
		out[i].Start = monday.AddDate(0, 0, 7*(i-weeks+1)).Format(time.DateOnly)
	}
	after := monday.AddDate(0, 0, 7).Format(time.DateOnly)
	// in is the week a date falls in, or nil outside the twelve. Dates are
	// written YYYY-MM-DD, so their string order is their calendar order.
	in := func(date string) *Week {
		if date == "" || date >= after {
			return nil
		}
		for i := weeks - 1; i >= 0; i-- {
			if date >= out[i].Start {
				return &out[i]
			}
		}
		return nil
	}

	// count tallies a Task and its Subtasks and gives back the day it ended,
	// "" for none known.
	var count func(t *taskfile.Task) string
	count = func(t *taskfile.Task) string {
		ended := t.Attr("ended")
		if len(t.Subtasks) > 0 {
			ended = ""
			for _, sub := range t.Subtasks {
				ended = max(ended, count(sub))
			}
		}
		if !tagged.keeps(Task{Tags: t.Tags}) {
			return ended
		}
		if w := in(t.Created); w != nil {
			w.Added++
		}
		if w := in(ended); w != nil && t.State.Ended() {
			if t.State == taskfile.Done {
				w.Done++
			} else {
				w.Declined++
			}
		}
		return ended
	}
	for _, r := range found {
		text, err := os.ReadFile(r.File())
		if err != nil {
			continue // the read reports it, as a problem on the Repo
		}
		f, _ := taskfile.Parse(string(text))
		for _, t := range f.Tasks {
			count(t)
		}
	}
	return out, nil
}
