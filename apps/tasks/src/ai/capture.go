package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Capture is one Task as the broker read it out of a brain dump: the whole
// Task as it should end up, said in the words somebody typed rather than in
// the shape a form asks for. It is a proposal like any other -- it becomes a
// Task where a person approves it and nowhere else -- and it is also the shape
// the broker is shown a Task in when a dump amends one, because a whole Task
// going out and a whole Task coming back are the same thing.
//
// A Repo and a Tag are named here rather than identified: an id means nothing
// to the broker, and the names it may choose from go with the dump. The Repo is
// a guess, and one guess: a Task is in exactly one Repo.
type Capture struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Why         string   `json:"why,omitempty"`
	Deadline    string   `json:"deadline,omitempty"`
	Estimate    string   `json:"estimate,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Impact      string   `json:"impact,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// Dump is what the broker is asked to read: somebody's words, today's date so
// a "tomorrow" in them lands somewhere, the Repo and Tag names it may choose
// from, and the Task as it stands when the dump amends one. The date is the
// caller's, because nothing in this package reads a clock.
type Dump struct {
	Text  string
	Today string
	Repos []string
	Tags  []string

	// Was is the Task the dump amends, and nil for a new one. The broker is
	// asked for the whole Task either way, so an attribute the dump says
	// nothing about comes back the way it went in.
	Was *Capture
}

const reading = `You read somebody's brain dump and fill in one task for a task tracker.

Answer with JSON and nothing else, shaped:
{"title": "...", "description": "...", "why": "...", "deadline": "2006-01-02",
"estimate": "small|medium|large", "priority": "low|med|high",
"impact": "low|med|high", "repo": "...", "tags": ["..."]}

title is one line with a verb in it. description is whatever else the dump
says, and does not repeat the title. why is only what the dump gives a reason
for. deadline is a date, 2006-01-02, worked out against today's date below.
estimate is small, medium or large. repo is the one repo it most likely
belongs in and tags are what it carries, both chosen from the names offered
and from nothing else.
Leave a field out rather than inventing it.

When the task is given as it stands, answer with the whole of it as it should
end up, changing only what the dump asks to change.`

// Read turns a dump into a Capture. It is one turn and no conversation: the
// broker is told everything at once and answers once, and what it answers is
// shown to somebody on a form before any of it is written.
func (c *Client) Read(ctx context.Context, d Dump) (Capture, error) {
	turn := "Today is " + d.Today + "."
	if len(d.Repos) > 0 {
		turn += "\nRepos it may go in: " + strings.Join(d.Repos, ", ")
	}
	if len(d.Tags) > 0 {
		turn += "\nTags it may carry: " + strings.Join(d.Tags, ", ")
	}
	if d.Was != nil {
		was, _ := json.Marshal(d.Was)
		turn += "\n\nThe task as it stands:\n" + string(was)
	}
	turn += "\n\nThe dump:\n" + d.Text

	said, err := c.complete(ctx, reading, turn, true)
	if err != nil {
		return Capture{}, err
	}
	var read Capture
	if err := json.Unmarshal([]byte(object(said)), &read); err != nil {
		return Capture{}, fmt.Errorf("the broker answered with something that is not a task: %w", err)
	}
	return read, nil
}
