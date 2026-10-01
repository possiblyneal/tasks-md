package write

import (
	"regexp"
	"strings"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/history"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// Entry is one write in a Task's history. Actor is "" for a direct edit,
// whose Actor nobody knows.
type Entry struct {
	At      time.Time `json:"at"`
	Actor   string    `json:"actor"`
	Subject string    `json:"subject"`
}

// HistoryOf is what happened to the Task with the id in dir's TASKS.md,
// newest first: each write whose subject names ^id, and each direct edit that
// changed one of the Task's own lines. A Subtask's lines are its own, so a
// hand edit to a Subtask is drawn on the Subtask alone. A Repo with no tasks
// history yet answers none. It writes nothing.
func HistoryOf(dir, id string) ([]Entry, error) {
	f, err := read(dir)
	if err != nil {
		return nil, err
	}
	if find(f, id) == nil {
		return nil, NoTask(id)
	}
	log, err := history.Log(dir)
	if err != nil {
		return nil, err
	}
	named := regexp.MustCompile(`\^` + regexp.QuoteMeta(id) + `\b`)
	entries := []Entry{}
	for _, c := range log {
		if c.Actor == history.DirectEdit {
			touched, err := touched(dir, c, id)
			if err != nil {
				return nil, err
			}
			if touched {
				entries = append(entries, Entry{At: c.At, Subject: c.Subject})
			}
			continue
		}
		if named.MatchString(c.Subject) {
			entries = append(entries, Entry{At: c.At, Actor: c.Actor, Subject: c.Subject})
		}
	}
	return entries, nil
}

// touched says whether the commit changed the Task's own lines, which a Task
// first appearing or vanishing does too.
func touched(dir string, c history.Commit, id string) (bool, error) {
	before, err := history.Show(dir, c.Parent)
	if err != nil {
		return false, err
	}
	after, err := history.Show(dir, c.Hash)
	if err != nil {
		return false, err
	}
	was, wasThere := own(before, id)
	is, isThere := own(after, id)
	return wasThere != isThere || was != is, nil
}

// own is the text of the Task's own lines as the file holds them: its title
// line up to the next Task's, trailing blank lines left off. The lines are
// read as written rather than canonically, so a hand edit to spacing alone
// counts.
func own(text, id string) (string, bool) {
	f, _ := taskfile.Parse(text)
	var order []*taskfile.Task
	walkPaths(f, func(path trail) { order = append(order, path.task()) })
	lines := strings.Split(text, "\n")
	for i, t := range order {
		if t.ID != id {
			continue
		}
		end := len(lines)
		if i+1 < len(order) {
			end = order[i+1].Line - 1
		}
		return strings.TrimRight(strings.Join(lines[t.Line-1:end], "\n"), " \n"), true
	}
	return "", false
}
