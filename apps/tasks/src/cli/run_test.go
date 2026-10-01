package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errs bytes.Buffer
	code = Run(args, &out, &errs)
	return code, out.String(), errs.String()
}

const houseMove = `# Tasks
color: green

- [ ] Pack the kitchen | doing #packing #kitchen
  - id: m3qa
  - created: 2026-09-20

  Start with the glassware.

  - [x] Buy boxes | done
    - id: m3qb
    - created: 2026-09-20
  - [ ] Wrap glassware | doing #kitchen
    - id: m3qc
    - created: 2026-09-20
- [ ] Book the van | backlog
  - id: v9t1
  - created: 2026-09-21
  - blocked by: m3qa
`

const work = `# Tasks

- [ ] Write the report | inbox #writing
  - id: r3pt
  - created: 2026-09-22
`

// home gives the test a home of its own, with no config, so the one root is
// ~/code, and writes each Repo's TASKS.md into it. It returns ~/code.
func home(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	code := filepath.Join(dir, "code")
	for repo, text := range files {
		place(t, filepath.Join(code, repo), text)
	}
	if err := os.MkdirAll(code, 0o755); err != nil {
		t.Fatal(err)
	}
	return code
}

func place(t *testing.T, dir, text string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListShowsEveryRepoGrouped(t *testing.T) {
	home(t, map[string]string{"house-move": houseMove, "work": work})

	code, out, errs := run(t, "list")
	if code != 0 {
		t.Fatalf("tasks list exited %d: %s", code, errs)
	}
	house, rest, ok := strings.Cut(out, "\nwork ")
	if !ok || !strings.HasPrefix(house, "house-move ") {
		t.Fatalf("tasks list = %q, want house-move's group then work's", out)
	}
	for _, want := range []string{"m3qa", "m3qb", "m3qc", "v9t1"} {
		if !strings.Contains(house, want) {
			t.Errorf("house-move's group = %q, want %s", house, want)
		}
	}
	if !strings.Contains(rest, "r3pt inbox Write the report #writing") {
		t.Errorf("work's group = %q, want the report", rest)
	}
	if !strings.Contains(house, "v9t1 backlog Book the van blocked") {
		t.Errorf("house-move's group = %q, want the van marked blocked", house)
	}
}

func TestListNarrows(t *testing.T) {
	home(t, map[string]string{"house-move": houseMove, "work": work})
	every := []string{"m3qa", "m3qb", "m3qc", "v9t1", "r3pt"}
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"-repo", "work"}, []string{"r3pt"}},
		{[]string{"-state", "doing"}, []string{"m3qa", "m3qc"}},
		{[]string{"-state", "done", "-state", "inbox"}, []string{"m3qb", "r3pt"}},
		{[]string{"-tag", "kitchen"}, []string{"m3qa", "m3qc"}},
		{[]string{"-tag", "writing", "-tag", "packing"}, []string{"m3qa", "r3pt"}},
		{[]string{"-search", "GLASS"}, []string{"m3qa", "m3qc"}},
		{[]string{"-unblocked"}, []string{"m3qa", "m3qb", "m3qc", "r3pt"}},
		{[]string{"-blocked"}, []string{"v9t1"}},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			code, out, errs := run(t, append([]string{"list"}, c.args...)...)
			if code != 0 {
				t.Fatalf("exited %d: %s", code, errs)
			}
			for _, id := range every {
				in := strings.Contains(out, " "+id+" ")
				if want := slices.Contains(c.want, id); in != want {
					t.Errorf("%s in the list = %v, want %v:\n%s", id, in, want, out)
				}
			}
		})
	}
}

func TestListJSONIsWhatAnAgentReads(t *testing.T) {
	home(t, map[string]string{"house-move": houseMove, "work": work})

	code, out, errs := run(t, "list", "-json", "-repo", "house-move")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, errs)
	}
	var body struct {
		Repos []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
			Tasks []struct {
				ID      string   `json:"id"`
				State   string   `json:"state"`
				Parents []string `json:"parents"`
				Leaf    bool     `json:"leaf"`
				Blocked bool     `json:"blocked"`
			} `json:"tasks"`
		} `json:"repos"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if len(body.Repos) != 1 || body.Repos[0].Name != "house-move" || body.Repos[0].Color != "green" {
		t.Fatalf("repos = %+v", body.Repos)
	}
	tasks := body.Repos[0].Tasks
	if len(tasks) != 4 {
		t.Fatalf("tasks = %+v", tasks)
	}
	wrap := tasks[2]
	if wrap.ID != "m3qc" || !wrap.Leaf || len(wrap.Parents) != 1 || wrap.Parents[0] != "Pack the kitchen" {
		t.Errorf("wrap = %+v, want a leaf under Pack the kitchen", wrap)
	}
	if pack := tasks[0]; pack.Leaf || pack.State != "doing" {
		t.Errorf("pack = %+v, want a doing parent", pack)
	}
	if van := tasks[3]; !van.Blocked {
		t.Errorf("van = %+v, want blocked by the kitchen", van)
	}
	if body.Errors == nil {
		t.Error("errors = null, want []")
	}
}

// A sort orders siblings and never flattens the tree, so a Subtask still sits
// under its parent; and every Task carries its rank across every Repo, which
// is the order a lane spanning several files is drawn in.
func TestListSorts(t *testing.T) {
	home(t, map[string]string{"house-move": `# Tasks

- [ ] Pack the kitchen | doing
  - id: m3qa
  - created: 2026-09-20
  - [ ] Wrap glassware | doing
    - id: m3qc
    - created: 2026-09-20
    - deadline: 2026-10-09
  - [ ] Label boxes | backlog
    - id: m3qd
    - created: 2026-09-20
    - deadline: 2026-10-02
- [ ] Book the van | backlog
  - id: v9t1
  - created: 2026-09-21
  - priority: high
  - estimate: S
`, "work": `# Tasks

- [ ] Write the report | inbox
  - id: r3pt
  - created: 2026-09-22
  - deadline: 2026-10-05
  - priority: low
  - estimate: L
`})
	order := func(args ...string) string {
		t.Helper()
		code, out, errs := run(t, append([]string{"list"}, args...)...)
		if code != 0 {
			t.Fatalf("exited %d: %s", code, errs)
		}
		var ids []string
		for _, line := range strings.Split(out, "\n") {
			if fields := strings.Fields(line); len(fields) > 1 && len(fields[0]) == 4 && !strings.Contains(line, "/") {
				ids = append(ids, fields[0])
			}
		}
		return strings.Join(ids, " ")
	}
	cases := map[string]string{
		"":         "m3qa m3qc m3qd v9t1 r3pt",
		"file":     "m3qa m3qc m3qd v9t1 r3pt",
		"title":    "v9t1 m3qa m3qd m3qc r3pt",
		"deadline": "m3qa m3qd m3qc v9t1 r3pt",
		"priority": "v9t1 m3qa m3qc m3qd r3pt",
		"estimate": "v9t1 m3qa m3qc m3qd r3pt",
		"created":  "m3qa m3qc m3qd v9t1 r3pt",
	}
	for sort, want := range cases {
		args := []string{}
		if sort != "" {
			args = []string{"-sort", sort}
		}
		if got := order(args...); got != want {
			t.Errorf("-sort %q = %s, want %s", sort, got, want)
		}
	}

	code, out, errs := run(t, "list", "-json", "-sort", "deadline")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, errs)
	}
	var body struct {
		Repos []struct {
			Tasks []struct {
				ID   string `json:"id"`
				Rank int    `json:"rank"`
			} `json:"tasks"`
		} `json:"repos"`
		Sorts []string `json:"sorts"`
		Today string   `json:"today"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	ranks := map[string]int{}
	for _, r := range body.Repos {
		for _, task := range r.Tasks {
			ranks[task.ID] = task.Rank
		}
	}
	// By deadline across both files, the ones with none after the rest.
	want := map[string]int{"m3qd": 0, "r3pt": 1, "m3qc": 2, "m3qa": 3, "v9t1": 4}
	for id, rank := range want {
		if ranks[id] != rank {
			t.Errorf("rank of %s = %d, want %d (%v)", id, ranks[id], rank, ranks)
		}
	}
	if strings.Join(body.Sorts, " ") != "file title deadline created priority estimate" {
		t.Errorf("sorts = %v", body.Sorts)
	}
	if body.Today != time.Now().Format(time.DateOnly) {
		t.Errorf("today = %q, want the host's local date", body.Today)
	}

	if code, _, errs := run(t, "list", "-sort", "colour"); code != 2 || !strings.Contains(errs, "colour") {
		t.Errorf("an unknown sort exited %d (%s), want 2 naming it", code, errs)
	}
}

func TestListRefusesWhatItCannotAnswer(t *testing.T) {
	home(t, map[string]string{"work": work})
	if code, _, errs := run(t, "list", "-repo", "nowhere"); code != 1 || !strings.Contains(errs, "nowhere") {
		t.Errorf("an unknown Repo exited %d (%s), want 1 naming it", code, errs)
	}
	if code, _, errs := run(t, "list", "-state", "someday"); code != 2 || !strings.Contains(errs, "someday") {
		t.Errorf("an unknown State exited %d (%s), want 2 naming it", code, errs)
	}
	if code, _, errs := run(t, "list", "-blocked", "-unblocked"); code != 2 || !strings.Contains(errs, "Blocked") {
		t.Errorf("-blocked with -unblocked exited %d saying %q, want 2 naming the clash", code, errs)
	}
}

func TestReposListsWhereEachLivesAndItsCounts(t *testing.T) {
	code := home(t, map[string]string{"house-move": houseMove, ".hidden": work})
	elsewhere, _ := filepath.EvalSymlinks(t.TempDir())
	place(t, filepath.Join(elsewhere, "work"), work)
	if err := os.Symlink(filepath.Join(elsewhere, "work"), filepath.Join(code, "work")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(code, "house-move"), filepath.Join(code, "house-move-link")); err != nil {
		t.Fatal(err)
	}

	status, out, errs := run(t, "repos")
	if status != 0 {
		t.Fatalf("tasks repos exited %d: %s", status, errs)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("tasks repos = %q, want a header and two Repos", out)
	}
	house := strings.Fields(lines[1])
	if strings.Join(house, " ") != "house-move "+filepath.Join(code, "house-move")+" 0 1 2 0 1 0 not backed up" {
		t.Errorf("house-move = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "work") || !strings.Contains(lines[2], filepath.Join(elsewhere, "work")) {
		t.Errorf("work = %q, want the linked folder's real path", lines[2])
	}
}

func TestOneNameUnderTwoRootsIsAConfigError(t *testing.T) {
	code := home(t, map[string]string{"notes": work})
	other, _ := filepath.EvalSymlinks(t.TempDir())
	place(t, filepath.Join(other, "notes"), work)
	config := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "tasks")
	if err := os.MkdirAll(config, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "root = ~/code\nroot = " + other + "\n"
	if err := os.WriteFile(filepath.Join(config, "config"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	status, _, errs := run(t, "repos")
	if status != 1 {
		t.Errorf("tasks repos exited %d, want 1", status)
	}
	for _, path := range []string{filepath.Join(code, "notes"), filepath.Join(other, "notes")} {
		if !strings.Contains(errs, path) {
			t.Errorf("stderr = %q, want it to name %s", errs, path)
		}
	}
}

// The conflict markers are spliced in so the file itself never holds one at the
// start of a line, which the merge-conflict hook would take for a real one.
const broken = `# Tasks

` + "<<<<<<< HEAD" + `
- [ ] Mine | inbox
  - id: aaaa
  - created: 2026-09-20
` + "=======\n>>>>>>> origin/tasks" + `
- [ ] Twice | inbox
  - id: aaaa
  - created: 2026-09-20
  - blocked by: zzzz
- [ ] Hand typed | wishful
- [x] Parent | done
  - id: pppp
  - created: 2026-09-20
  - [ ] Child | doing
    - id: cccc
    - created: 2026-09-20
`

func TestLintReportsEveryProblemByLine(t *testing.T) {
	code := home(t, map[string]string{"work": work, "broken": broken})
	file := filepath.Join(code, "broken", "TASKS.md")

	status, out, errs := run(t, "lint")
	if status != 1 {
		t.Fatalf("tasks lint exited %d, want 1: %s", status, errs)
	}
	for _, want := range []string{
		file + ":3: conflict marker",
		file + ":7: conflict marker",
		file + ":8: conflict marker",
		file + ":10: id aaaa is also on line 5",
		file + ":12: blocked by zzzz",
		file + `:13: "wishful" is not a State`,
		file + `:13: "Hand typed" has no id line`,
		file + `:13: "Hand typed" has no created line`,
		file + ":14: the line says done, but its Subtasks make it doing",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("tasks lint = %q, want %q", out, want)
		}
	}
	if strings.Contains(out, "work") {
		t.Errorf("tasks lint = %q, want nothing about the clean Repo", out)
	}
}

func TestLintIsQuietOverACleanFileNamedOnTheCommandLine(t *testing.T) {
	code := home(t, map[string]string{"work": work})
	status, out, errs := run(t, "lint", filepath.Join(code, "work", "TASKS.md"))
	if status != 0 || out != "" || errs != "" {
		t.Errorf("tasks lint exited %d with %q %q, want 0 and nothing", status, out, errs)
	}
}

func TestUnknownVerbIsRefused(t *testing.T) {
	code, _, errs := run(t, "frobnicate")
	if code != 2 {
		t.Errorf("an unknown verb exited %d, want 2", code)
	}
	if !strings.Contains(errs, "frobnicate") {
		t.Errorf("stderr = %q, want it to name the unknown verb", errs)
	}
}

// The bare invocation says what the binary is for, and names every verb the
// dispatch holds, because the two read one list.
func TestBareTasksSaysWhatItIsFor(t *testing.T) {
	code, out, errs := run(t)
	if code != 2 {
		t.Fatalf("bare tasks exited %d, want 2: %s%s", code, out, errs)
	}
	for _, want := range []string{"tasks <verb>", "tasks api"} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr = %q, want it to name %q", errs, want)
		}
	}
	for _, one := range verbs {
		if !strings.Contains(errs, one.verb) {
			t.Errorf("stderr = %q, want it to name the verb %q", errs, one.verb)
		}
	}
}

// `tasks serve` is gone with the TUI it served: an unknown verb like any other.
func TestServeIsGone(t *testing.T) {
	code, _, errs := run(t, "serve")
	if code != 2 {
		t.Fatalf("tasks serve exited %d, want 2", code)
	}
	if !strings.Contains(errs, "unknown verb") {
		t.Errorf("stderr = %q, want it to call serve an unknown verb", errs)
	}
}
