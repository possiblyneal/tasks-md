package taskfile

import (
	"fmt"
	"strings"
	"testing"
)

// outline draws a parsed tree one Task a line, so a table can say what it
// expects as text rather than as nested structs.
func outline(tasks []*Task, depth int, b *strings.Builder) {
	for _, t := range tasks {
		fmt.Fprintf(b, "%s%s %s %s", strings.Repeat("  ", depth), t.ID, t.State, t.Title)
		for _, tag := range t.Tags {
			b.WriteString(" #" + tag)
		}
		b.WriteString("\n")
		outline(t.Subtasks, depth+1, b)
	}
}

func drawn(f *File) string {
	var b strings.Builder
	outline(f.Tasks, 0, &b)
	return b.String()
}

// said renders problems as "line: message" so a table can name them.
func said(problems []Problem) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = fmt.Sprintf("%d: %s", p.Line, p.Message)
	}
	return out
}

const header = "# Tasks\ncolor: green\n<!-- Agents: use the tasks verbs. -->\n\n"

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		text string
		tree string
		// Each problem is "line: words the message must contain".
		problems []string
	}{
		{
			name: "title line, fixed lines and tags",
			text: "- [ ] Pack the kitchen | doing #packing #kitchen\n  - id: m3qa\n  - created: 2026-09-20\n",
			tree: "m3qa doing Pack the kitchen #packing #kitchen\n",
		},
		{
			name: "the tail is split at the last bar",
			text: "- [ ] Either | or | backlog\n  - id: e1th\n  - created: 2026-09-20\n",
			tree: "e1th backlog Either | or\n",
		},
		{
			name: "the checkbox wins over the word",
			text: "- [x] Ticked | doing\n  - id: aaaa\n  - created: 2026-09-20\n" +
				"- [-] Struck | inbox\n  - id: bbbb\n  - created: 2026-09-20\n" +
				"- [ ] Unticked | done\n  - id: cccc\n  - created: 2026-09-20\n",
			tree: "aaaa done Ticked\nbbbb declined Struck\ncccc backlog Unticked\n",
		},
		{
			name: "no word is Inbox",
			text: "- [ ] Buy milk\n  - id: m1lk\n  - created: 2026-09-20\n",
			tree: "m1lk inbox Buy milk\n",
		},
		{
			name: "a parent's State is worked out from its Subtasks",
			text: "- [ ] Move | doing\n  - id: p000\n  - created: 2026-09-20\n" +
				"  - [x] Boxes | done\n    - id: p001\n    - created: 2026-09-20\n" +
				"  - [ ] Van | backlog\n    - id: p002\n    - created: 2026-09-20\n",
			tree:     "p000 backlog Move\n  p001 done Boxes\n  p002 backlog Van\n",
			problems: []string{"1: says doing"},
		},
		{
			name: "a parent whose Subtasks all declined is Declined",
			text: "- [x] Sell | done\n  - id: s000\n  - created: 2026-09-20\n" +
				"  - [-] Sofa | declined\n    - id: s001\n    - created: 2026-09-20\n",
			tree:     "s000 declined Sell\n  s001 declined Sofa\n",
			problems: []string{"1: says done"},
		},
		{
			name:     "an unknown State word",
			text:     "- [ ] Wander | someday\n  - id: w000\n  - created: 2026-09-20\n",
			problems: []string{"1: someday"},
		},
		{
			name:     "a tail word that is not a tag",
			text:     "- [ ] Odd | doing kitchen\n  - id: o000\n  - created: 2026-09-20\n",
			problems: []string{"1: kitchen"},
		},
		{
			name:     "missing id and created",
			text:     "- [ ] Hand typed | inbox\n",
			tree:     " inbox Hand typed\n",
			problems: []string{"1: no id", "1: no created"},
		},
		{
			name:     "created before id",
			text:     "- [ ] Swapped | inbox\n  - created: 2026-09-20\n  - id: sw00\n",
			problems: []string{"2: created", "3: id"},
		},
		{
			name:     "a malformed id",
			text:     "- [ ] Shouty | inbox\n  - id: ABCDE\n  - created: 2026-09-20\n",
			problems: []string{"2: ABCDE"},
		},
		{
			name: "duplicate ids",
			text: "- [ ] One | inbox\n  - id: dup0\n  - created: 2026-09-20\n" +
				"- [ ] Two | inbox\n  - id: dup0\n  - created: 2026-09-20\n",
			problems: []string{"5: dup0 is also on line 6"}, // 2, after the header
		},
		{
			name:     "an unknown blocker",
			text:     "- [ ] Van | backlog\n  - id: v9t1\n  - created: 2026-09-20\n  - blocked by: q4ha, v9t1\n",
			problems: []string{"4: q4ha"},
		},
		{
			name: "conflict markers",
			text: "<<<<<<< HEAD\n- [ ] Mine | inbox\n  - id: mine\n  - created: 2026-09-20\n=======\n" +
				"- [ ] Theirs | inbox\n  - id: thrs\n  - created: 2026-09-20\n>>>>>>> origin/tasks\n",
			problems: []string{"1: conflict", "5: conflict", "9: conflict"},
		},
		{
			name:     "a line that is nothing this format writes",
			text:     "- [ ] Fine | inbox\n  - id: fine\n  - created: 2026-09-20\n## Done\n",
			problems: []string{"4: cannot read"},
		},
		{
			name:     "a Subtask indented past its parent",
			text:     "- [ ] Top | inbox\n  - id: top0\n  - created: 2026-09-20\n      - [ ] Lost | inbox\n",
			problems: []string{"4: indented"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, problems := Parse(header + c.text)
			// The header is four lines, so a table's line numbers are
			// counted from the first Task.
			got := said(problems)
			if len(got) != len(c.problems) {
				t.Fatalf("problems = %q, want %d like %q", got, len(c.problems), c.problems)
			}
			for i, want := range c.problems {
				line, words, _ := strings.Cut(want, ": ")
				var n int
				fmt.Sscan(line, &n)
				if problems[i].Line != n+4 || !strings.Contains(problems[i].Message, words) {
					t.Errorf("problem %d = %q, want line %d containing %q", i, got[i], n+4, words)
				}
			}
			if c.tree != "" {
				if d := drawn(f); d != c.tree {
					t.Errorf("tree =\n%s\nwant\n%s", d, c.tree)
				}
			}
		})
	}
}

func TestParseReadsTheHeaderAttributesDescriptionAndSubtasks(t *testing.T) {
	f, problems := Parse(header + canonical)
	if len(problems) != 0 {
		t.Fatalf("problems = %q", said(problems))
	}
	if f.Color() != "green" {
		t.Errorf("color = %q, want green", f.Color())
	}
	pack := f.Tasks[0]
	if pack.Line != 5 || pack.Created != "2026-09-20" {
		t.Errorf("line, created = %d, %q", pack.Line, pack.Created)
	}
	if pack.Attr("deadline") != "2026-10-10" || pack.Attr("mood") != "hopeful" {
		t.Errorf("attrs = %v", pack.Attrs)
	}
	if pack.Description != "Start with the glassware.\n\nKeep one pan out." {
		t.Errorf("description = %q", pack.Description)
	}
	if len(pack.Subtasks) != 2 || pack.Subtasks[1].Title != "Wrap glassware" {
		t.Fatalf("subtasks = %v", pack.Subtasks)
	}
	van := f.Tasks[1]
	if got := van.BlockedBy(); len(got) != 1 || got[0] != "m3qa" {
		t.Errorf("blocked by = %q, want [m3qa]", got)
	}
}

// canonical is a file the way Write writes one, so it must come back as it
// went in, unknown labels and all.
const canonical = `- [ ] Pack the kitchen | doing #packing #kitchen
  - id: m3qa
  - created: 2026-09-20
  - deadline: 2026-10-10
  - mood: hopeful

  Start with the glassware.

  Keep one pan out.

  - [x] Buy boxes and tape | done
    - id: m3qb
    - created: 2026-09-20
  - [ ] Wrap glassware | doing
    - id: m3qc
    - created: 2026-09-21
- [ ] Book the van | backlog
  - id: v9t1
  - created: 2026-09-21
  - blocked by: m3qa
- [-] Sell the sofa | declined
  - id: s0fa
  - created: 2026-09-21
  - reason: it fits after all
`

func TestWriteGivesBackACanonicalFileUnchanged(t *testing.T) {
	f, problems := Parse(header + canonical)
	if len(problems) != 0 {
		t.Fatalf("problems = %q", said(problems))
	}
	if got := Write(f); got != header+canonical {
		t.Errorf("round trip changed the file:\n%s", got)
	}
}

func TestWriteRewritesAHandEditCanonically(t *testing.T) {
	hand := "- [x]  Ticked   | doing #a\n  - id: aaaa\n  - created: 2026-09-20\n\n\n" +
		"- [ ] Parent | done\n  - id: bbbb\n  - created: 2026-09-20\n" +
		"  - [ ] Child\n    - id: cccc\n    - created: 2026-09-20\n"
	want := "- [x] Ticked | done #a\n  - id: aaaa\n  - created: 2026-09-20\n" +
		"- [ ] Parent | inbox\n  - id: bbbb\n  - created: 2026-09-20\n" +
		"  - [ ] Child | inbox\n    - id: cccc\n    - created: 2026-09-20\n"
	f, _ := Parse(header + hand)
	if got := Write(f); got != header+want {
		t.Errorf("canonical rewrite =\n%s\nwant\n%s", got, header+want)
	}
}

func TestVersionFollowsTheWholeTreeAndNothingElse(t *testing.T) {
	text := "# Tasks\n\n- [ ] Pack | doing\n  - id: m3qa\n  - created: 2026-09-20\n  - [ ] Wrap | doing\n    - id: m3qc\n    - created: 2026-09-20\n- [ ] Van | backlog\n  - id: v9t1\n  - created: 2026-09-21\n"
	f, _ := Parse(text)
	pack, van := Version(f.Tasks[0]), Version(f.Tasks[1])
	if pack == van || pack == "" {
		t.Fatalf("two trees share version %q", pack)
	}

	f.Tasks[0].Subtasks[0].Title = "Wrap the glasses"
	if Version(f.Tasks[0]) == pack {
		t.Errorf("changing a Subtask left its tree's version alone")
	}
	if Version(f.Tasks[1]) != van {
		t.Errorf("changing one tree changed another's version")
	}

	// Where a tree sits in the file is not part of it.
	moved, _ := Parse("# Tasks\n\nA new preamble line.\n\n" + text[len("# Tasks\n\n"):])
	if Version(moved.Tasks[1]) != van {
		t.Errorf("a tree's version changed with its line number")
	}
}
