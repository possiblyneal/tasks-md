package write

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/ai"
	"github.com/possiblyneal/tasks-md/apps/tasks/src/board"
)

// dumpDate is the date the Broker works a "tomorrow" out against. The weekday
// is part of it because "Friday" in a dump is a date only where the Broker is
// told which day today is.
const dumpDate = "2006-01-02, Monday"

// DumpOf is a dump ready for the Broker: the words, today, and every Repo and
// Tag name on the board, by name because an id means nothing to it.
func DumpOf(text string, read []board.Repo, now time.Time) ai.Dump {
	d := ai.Dump{Text: text, Today: now.Format(dumpDate), Tags: TagsOf(read)}
	for _, r := range read {
		d.Repos = append(d.Repos, r.Name)
	}
	return d
}

// TagsOf is every Tag a Task read carries, sorted, each once.
func TagsOf(read []board.Repo) []string {
	var tags []string
	for _, r := range read {
		for _, t := range r.Tasks {
			tags = append(tags, t.Tags...)
		}
	}
	slices.Sort(tags)
	return slices.Compact(tags)
}

// Briefs is every Task read, as the Broker is shown it: what it says, its
// Repo and Tags by name, and no id.
func Briefs(read []board.Repo) []ai.Brief {
	var out []ai.Brief
	for _, r := range read {
		for _, t := range r.Tasks {
			out = append(out, brief(r.Name, t))
		}
	}
	return out
}

// BriefOf is the one Task in the Repo read, or NoTask.
func BriefOf(r board.Repo, id string) (ai.Brief, error) {
	for _, t := range r.Tasks {
		if t.ID == id {
			return brief(r.Name, t), nil
		}
	}
	return ai.Brief{}, NoTask(id)
}

// WasOf is the one Task in the Repo read as a dump amending it carries it, or
// NoTask.
func WasOf(r board.Repo, id string) (*ai.Capture, error) {
	b, err := BriefOf(r, id)
	if err != nil {
		return nil, err
	}
	return &ai.Capture{
		Title:       b.Title,
		Description: b.Description,
		Why:         b.Why,
		Deadline:    b.Deadline,
		Estimate:    b.Estimate,
		Priority:    b.Priority,
		Impact:      b.Impact,
		Repo:        b.Repo,
		Tags:        b.Tags,
	}, nil
}

func brief(repo string, t board.Task) ai.Brief {
	attr := func(label string) string {
		for _, a := range t.Attrs {
			if a.Label == label {
				return a.Value
			}
		}
		return ""
	}
	return ai.Brief{
		Title:       t.Title,
		Repo:        repo,
		Tags:        t.Tags,
		Description: t.Description,
		Why:         attr("why"),
		Deadline:    attr("deadline"),
		Estimate:    attr("estimate"),
		Priority:    attr("priority"),
		Impact:      attr("impact"),
	}
}

// FromCapture is what the Broker read out of a dump as a Task to write, with
// what this program cannot read dropped rather than refused: `tasks capture`
// has nobody left to ask. A missing title is the one refusal, because a Task
// with no title is not one. Tags are the offered ones it chose; the Repo is
// the caller's, never the Broker's guess.
func FromCapture(read ai.Capture, tags []string) (New, error) {
	n := readable(New{
		Title:       read.Title,
		Description: read.Description,
		Why:         read.Why,
		Deadline:    read.Deadline,
		Estimate:    read.Estimate,
		Priority:    read.Priority,
		Impact:      read.Impact,
		Tags:        NamedIn(read.Tags, tags),
	})
	if n.Title == "" {
		return New{}, errors.New("the broker read no title out of that")
	}
	return n, nil
}

// FromProposal is a proposed Subtask under the same rule, so a proposal
// shown with a tick beside it is one the store takes as shown.
func FromProposal(p ai.Proposal) New {
	return readable(New{
		Title:       p.Title,
		Description: p.Description,
		Why:         p.Why,
		Estimate:    p.Estimate,
		Priority:    p.Priority,
		Impact:      p.Impact,
	})
}

// readable drops each value the Broker said that check would refuse.
func readable(n New) New {
	n.Title = strings.Join(strings.Fields(n.Title), " ")
	n.Why = strings.Join(strings.Fields(n.Why), " ")
	if _, err := time.Parse(time.DateOnly, n.Deadline); err != nil {
		n.Deadline = ""
	}
	for _, v := range []struct {
		value *string
		from  []string
	}{{&n.Priority, levels}, {&n.Impact, levels}, {&n.Estimate, estimates}} {
		*v.value = strings.ToLower(strings.TrimSpace(*v.value))
		if !slices.Contains(v.from, *v.value) {
			*v.value = ""
		}
	}
	return n
}

// NamedIn is the offered names the Broker chose, matched ignoring case and
// spelled as offered. A name it invented is dropped, and one said twice is
// kept once.
func NamedIn(chosen, offered []string) []string {
	var out []string
	for _, name := range chosen {
		i := slices.IndexFunc(offered, func(o string) bool { return strings.EqualFold(o, strings.TrimSpace(name)) })
		if i >= 0 && !slices.Contains(out, offered[i]) {
			out = append(out, offered[i])
		}
	}
	return out
}
