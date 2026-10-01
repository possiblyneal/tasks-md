package write

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/history"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/schedule"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// shown is how many of a Series' next dates `tasks repeat` prints and the
// series route answers.
const shown = 5

// Series is a Task's rule as the file holds it and the next dates it produces
// from today on. Rule is "" and Dates empty for a Task that does not repeat.
type Series struct {
	Rule  string   `json:"rule"`
	Dates []string `json:"dates"`
}

// Repeat puts a Series on the Task with the id, replacing any it has, or
// takes it off when rule is "". The rule is written back the way
// schedule.String writes it, so it carries the date it runs from.
func Repeat(dir, actor, id, rule string) error {
	written, subject := "", "off"
	if rule != "" {
		parsed, err := schedule.Parse(rule)
		if err != nil {
			return Invalid{err}
		}
		written = parsed.String()
		subject = written
	}
	return history.Write(dir, actor, func(before string) (string, string, error) {
		f, err := parse(before)
		if err != nil {
			return "", "", err
		}
		path := find(f, id)
		if path == nil {
			return "", "", NoTask(id)
		}
		set(path[len(path)-1].task, "series", written)
		return taskfile.Write(f), "repeat ^" + id + " " + subject, nil
	})
}

// SeriesOf is the Series on the Task with the id in dir's tasks.md, with its
// next dates counted from today. It writes nothing.
func SeriesOf(dir, id string, today time.Time) (Series, error) {
	text, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
	if err != nil {
		return Series{}, err
	}
	f, _ := taskfile.Parse(string(text))
	path := find(f, id)
	if path == nil {
		return Series{}, NoTask(id)
	}
	s := Series{Rule: path[len(path)-1].task.Attr("series"), Dates: []string{}}
	if s.Rule == "" {
		return s, nil
	}
	rule, err := schedule.Parse(s.Rule)
	if err != nil {
		return Series{}, fmt.Errorf("^%s's series: %w", id, err)
	}
	for from := day(today); len(s.Dates) < shown; {
		next, ok := rule.Next(from)
		if !ok {
			break
		}
		s.Dates = append(s.Dates, next.Format(time.DateOnly))
		from = next.AddDate(0, 0, 1)
	}
	return s, nil
}

// String is the Series as `tasks repeat` prints it: the rule, then a date a
// line.
func (s Series) String() string {
	return strings.Join(append([]string{s.Rule}, s.Dates...), "\n") + "\n"
}

// day is the date a moment falls on, in the host's zone.
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}
