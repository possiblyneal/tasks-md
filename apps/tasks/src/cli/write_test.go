package cli

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// gitHome is home with a host git user, the one every tasks commit is
// authored as, and nothing else of the real machine's git config.
func gitHome(t *testing.T, files map[string]string) string {
	t.Helper()
	code := home(t, files)
	config := "[user]\n\tname = Host Person\n\temail = host@example.com\n[init]\n\tdefaultBranch = main\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("TASKS_ACTOR", "claude-code/claude-opus-5-5")
	return code
}

// git runs git in dir and gives back what it printed, failing the test on an
// error.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// withRemote makes dir a code repository whose origin is a fresh bare
// repository, and gives back the bare repository's path.
func withRemote(t *testing.T, dir string) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "remote.git")
	git(t, filepath.Dir(dir), "init", "-q", "--bare", bare)
	git(t, dir, "init", "-q")
	git(t, dir, "remote", "add", "origin", bare)
	return bare
}

// tasksLog is the Repo's tasks history, newest first, one line a commit in the
// format given.
func tasksLog(t *testing.T, dir, format string) []string {
	t.Helper()
	out := git(t, dir, "--git-dir=.tasks.git", "log", "--format="+format)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func read(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func today() string { return time.Now().Format(time.DateOnly) }

var anID = regexp.MustCompile(`^[a-z0-9]{4}$`)

func TestAddIsOneCommitPushedToTheTasksBranch(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	bare := withRemote(t, repo)
	// The file was there before the tracker ever wrote it, so it goes in as a
	// direct edit first, and the add is a commit of its own.
	status, out, errs := run(t, "add", "Measure the hallway", "-repo", "house-move", "-tag", "measuring")
	if status != 0 {
		t.Fatalf("tasks add exited %d: %s", status, errs)
	}
	id := strings.TrimSpace(out)
	if !anID.MatchString(id) {
		t.Fatalf("tasks add printed %q, want the new id", out)
	}

	want := "- [ ] Measure the hallway | inbox #measuring\n  - id: " + id + "\n  - created: " + today() + "\n"
	if file := read(t, filepath.Join(repo, "TASKS.md")); !strings.Contains(file, want) {
		t.Errorf("TASKS.md =\n%s\nwant it to hold\n%s", file, want)
	}

	commits := tasksLog(t, repo, "%s|%an <%ae>|%(trailers:key=Generated-By,valueonly,separator=)")
	wantCommits := []string{
		"chore(tasks): add ^" + id + " Measure the hallway|Host Person <host@example.com>|claude-code/claude-opus-5-5",
		"chore(tasks): commit a direct edit|Host Person <host@example.com>|direct edit",
	}
	if strings.Join(commits, "\n") != strings.Join(wantCommits, "\n") {
		t.Errorf("tasks history =\n%s\nwant\n%s", strings.Join(commits, "\n"), strings.Join(wantCommits, "\n"))
	}
	if files := git(t, repo, "--git-dir=.tasks.git", "show", "--format=", "--name-only", "HEAD"); files != "TASKS.md" {
		t.Errorf("the add committed %q, want TASKS.md alone", files)
	}
	if pushed, head := git(t, bare, "rev-parse", "tasks"), git(t, repo, "--git-dir=.tasks.git", "rev-parse", "HEAD"); pushed != head {
		t.Errorf("the remote's tasks branch is %s, want the add %s", pushed, head)
	}
}

// moving reports a move that should go through, failing the test when it does
// not.
func moving(t *testing.T, args ...string) {
	t.Helper()
	if status, _, errs := run(t, append([]string{"move"}, args...)...); status != 0 {
		t.Fatalf("tasks move %v exited %d: %s", args, status, errs)
	}
}

func TestEndingTheLastOpenSubtaskEndsItsParentAndSinksBoth(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)

	moving(t, "m3qc", "done", "-repo", "house-move")
	ended := `# Tasks
color: green

- [ ] Book the van | backlog
  - id: v9t1
  - created: 2026-09-21
  - blocked by: m3qa
- [x] Pack the kitchen | done #packing #kitchen
  - id: m3qa
  - created: 2026-09-20
  - ended: ` + today() + `

  Start with the glassware.

  - [x] Buy boxes | done
    - id: m3qb
    - created: 2026-09-20
  - [x] Wrap glassware | done #kitchen
    - id: m3qc
    - created: 2026-09-20
    - ended: ` + today() + `
`
	if got := read(t, filepath.Join(repo, "TASKS.md")); got != ended {
		t.Errorf("after moving m3qc to done, TASKS.md =\n%s\nwant\n%s", got, ended)
	}

	// Reopening it reopens the parent: each goes back above the ended Tasks
	// of its level, and neither keeps an ended date.
	moving(t, "m3qc", "backlog", "-repo", "house-move")
	reopened := `# Tasks
color: green

- [ ] Book the van | backlog
  - id: v9t1
  - created: 2026-09-21
  - blocked by: m3qa
- [ ] Pack the kitchen | backlog #packing #kitchen
  - id: m3qa
  - created: 2026-09-20

  Start with the glassware.

  - [ ] Wrap glassware | backlog #kitchen
    - id: m3qc
    - created: 2026-09-20
  - [x] Buy boxes | done
    - id: m3qb
    - created: 2026-09-20
`
	if got := read(t, filepath.Join(repo, "TASKS.md")); got != reopened {
		t.Errorf("after reopening m3qc, TASKS.md =\n%s\nwant\n%s", got, reopened)
	}

	subjects := tasksLog(t, repo, "%s")
	want := []string{"chore(tasks): move ^m3qc to Backlog", "chore(tasks): move ^m3qc to Done", "chore(tasks): commit a direct edit"}
	if strings.Join(subjects, "\n") != strings.Join(want, "\n") {
		t.Errorf("tasks history =\n%s\nwant\n%s", strings.Join(subjects, "\n"), strings.Join(want, "\n"))
	}
}

func TestAReasonIsHeldOnlyWhileDeferredOrDeclined(t *testing.T) {
	code := gitHome(t, map[string]string{"errands": `# Tasks

- [ ] Renew the passport | deferred
  - id: pp01
  - created: 2026-09-01
  - until: 2099-12-01
  - reason: waiting on photos
`})
	repo := filepath.Join(code, "errands")
	withRemote(t, repo)

	moving(t, "pp01", "backlog", "-repo", "errands")
	backlog := "# Tasks\n\n- [ ] Renew the passport | backlog\n  - id: pp01\n  - created: 2026-09-01\n"
	if got := read(t, filepath.Join(repo, "TASKS.md")); got != backlog {
		t.Errorf("leaving Deferred, TASKS.md =\n%s\nwant\n%s", got, backlog)
	}

	moving(t, "pp01", "declined", "-reason", "moving abroad", "-repo", "errands")
	declined := "# Tasks\n\n- [-] Renew the passport | declined\n  - id: pp01\n  - created: 2026-09-01\n  - reason: moving abroad\n  - ended: " + today() + "\n"
	if got := read(t, filepath.Join(repo, "TASKS.md")); got != declined {
		t.Errorf("declining, TASKS.md =\n%s\nwant\n%s", got, declined)
	}
}

func TestMovesTheRulesTurnAwayChangeNothing(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)
	moving(t, "v9t1", "doing", "-repo", "house-move")
	before := read(t, filepath.Join(repo, "TASKS.md"))
	commits := len(tasksLog(t, repo, "%h"))

	for _, c := range []struct {
		args   []string
		status int
		says   string
	}{
		{[]string{"v9t1", "doing"}, 3, "already in Doing"},
		{[]string{"m3qa", "done"}, 3, "worked out from its Subtasks"},
		{[]string{"zzzz", "done"}, 1, "no Task has the id zzzz"},
		{[]string{"v9t1", "finished"}, 2, "not a State"},
		{[]string{"v9t1", "inbox", "-reason", "why not"}, 2, "Deferred or Declined"},
		{[]string{"v9t1"}, 2, "want an id and a State"},
	} {
		status, _, errs := run(t, append(append([]string{"move"}, c.args...), "-repo", "house-move")...)
		if status != c.status || !strings.Contains(errs, c.says) {
			t.Errorf("tasks move %v exited %d saying %q, want %d saying %q", c.args, status, errs, c.status, c.says)
		}
	}
	if got := read(t, filepath.Join(repo, "TASKS.md")); got != before {
		t.Errorf("a refused move changed TASKS.md to\n%s", got)
	}
	if got := len(tasksLog(t, repo, "%h")); got != commits {
		t.Errorf("a refused move made %d commits", got-commits)
	}
}

// flagsOf is the FLAGS column `tasks repos` prints for the Repo.
func flagsOf(t *testing.T, name string) string {
	t.Helper()
	status, out, errs := run(t, "repos")
	if status != 0 {
		t.Fatalf("tasks repos exited %d: %s", status, errs)
	}
	for line := range strings.Lines(out) {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == name {
			return strings.Join(fields[8:], " ")
		}
	}
	t.Fatalf("tasks repos printed no %s:\n%s", name, out)
	return ""
}

func TestAPushThatFailsKeepsTheCommitAndIsRetriedOnTheNextWrite(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	bare := withRemote(t, repo)
	hook := filepath.Join(bare, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	status, _, errs := run(t, "move", "v9t1", "doing", "-repo", "house-move")
	if status != 0 || !strings.Contains(errs, "warning: not pushed") {
		t.Fatalf("a move whose push failed exited %d saying %q, want 0 warning it is not pushed", status, errs)
	}
	if subjects := tasksLog(t, repo, "%s"); subjects[0] != "chore(tasks): move ^v9t1 to Doing" {
		t.Errorf("the move was not kept as a commit: %q", subjects)
	}
	if got := flagsOf(t, "house-move"); got != "not pushed" {
		t.Errorf("tasks repos flags house-move %q, want not pushed", got)
	}

	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	moving(t, "v9t1", "backlog", "-repo", "house-move")
	if pushed, head := git(t, bare, "rev-parse", "tasks"), git(t, repo, "--git-dir=.tasks.git", "rev-parse", "HEAD"); pushed != head {
		t.Errorf("the next write did not push: remote %s, history %s", pushed, head)
	}
	if got := flagsOf(t, "house-move"); got != "-" {
		t.Errorf("tasks repos flags house-move %q once pushed, want -", got)
	}
}

func TestAFolderWithNoRemoteIsNotBackedUp(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")

	status, _, errs := run(t, "move", "v9t1", "doing", "-repo", "house-move")
	if status != 0 || !strings.Contains(errs, "warning: not backed up") {
		t.Fatalf("a move with nowhere to push exited %d saying %q, want 0 warning it is not backed up", status, errs)
	}
	if subjects := tasksLog(t, repo, "%s"); subjects[0] != "chore(tasks): move ^v9t1 to Doing" {
		t.Errorf("the move was not committed: %q", subjects)
	}
	if got := flagsOf(t, "house-move"); got != "not backed up" {
		t.Errorf("tasks repos flags house-move %q, want not backed up", got)
	}
}

func TestAHandEditIsCommittedAsADirectEditBeforeTheWrite(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)
	moving(t, "v9t1", "doing", "-repo", "house-move")

	edited := strings.Replace(read(t, filepath.Join(repo, "TASKS.md")), "Book the van", "Book the big van", 1)
	place(t, repo, edited)
	moving(t, "v9t1", "backlog", "-repo", "house-move")

	subjects := tasksLog(t, repo, "%s|%(trailers:key=Generated-By,valueonly,separator=)")
	want := []string{
		"chore(tasks): move ^v9t1 to Backlog|claude-code/claude-opus-5-5",
		"chore(tasks): commit a direct edit|direct edit",
		"chore(tasks): move ^v9t1 to Doing|claude-code/claude-opus-5-5",
		"chore(tasks): commit a direct edit|direct edit",
	}
	if strings.Join(subjects, "\n") != strings.Join(want, "\n") {
		t.Errorf("tasks history =\n%s\nwant\n%s", strings.Join(subjects, "\n"), strings.Join(want, "\n"))
	}
	if diff := git(t, repo, "--git-dir=.tasks.git", "show", "--format=", "HEAD~1"); !strings.Contains(diff, "+- [ ] Book the big van") {
		t.Errorf("the direct edit's commit is\n%s\nwant the hand edit", diff)
	}
}

func TestTheCodeHistoryIgnoresTheFileAndTheTasksHistory(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("node_modules/"), 0o644); err != nil {
		t.Fatal(err)
	}
	moving(t, "v9t1", "doing", "-repo", "house-move")
	moving(t, "v9t1", "backlog", "-repo", "house-move")

	if got := read(t, filepath.Join(repo, ".gitignore")); got != "node_modules/\n/TASKS.md\n/.tasks.git\n" {
		t.Errorf(".gitignore = %q, want both appended once", got)
	}
	if status := git(t, repo, "status", "--porcelain"); status != "?? .gitignore" {
		t.Errorf("the code history sees %q, want only the .gitignore", status)
	}
}

func TestARepoInAMessRefusesWritesAndIsFlagged(t *testing.T) {
	for _, c := range []struct {
		name  string
		mess  func(t *testing.T, repo string)
		flags string
	}{
		{"conflict markers", func(t *testing.T, repo string) {
			place(t, repo, houseMove+"<<<<<<< ours\n=======\n>>>>>>> theirs\n")
		}, "refusing writes: TASKS.md holds conflict markers"},
		{"mid-merge", func(t *testing.T, repo string) {
			head := git(t, repo, "--git-dir=.tasks.git", "rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(repo, ".tasks.git", "MERGE_HEAD"), []byte(head+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "refusing writes: its tasks history is mid-merge"},
		{"mid-rebase", func(t *testing.T, repo string) {
			if err := os.Mkdir(filepath.Join(repo, ".tasks.git", "rebase-merge"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "refusing writes: its tasks history is mid-rebase"},
		{"detached", func(t *testing.T, repo string) {
			git(t, repo, "--git-dir=.tasks.git", "--work-tree=.", "checkout", "-q", "--detach")
		}, "refusing writes: its tasks history is on a detached HEAD"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code := gitHome(t, map[string]string{"house-move": houseMove})
			repo := filepath.Join(code, "house-move")
			withRemote(t, repo)
			moving(t, "v9t1", "doing", "-repo", "house-move")
			c.mess(t, repo)
			before := read(t, filepath.Join(repo, "TASKS.md"))

			status, _, errs := run(t, "move", "v9t1", "backlog", "-repo", "house-move")
			if status != 3 || !strings.Contains(errs, c.flags) {
				t.Errorf("a write to a Repo %s exited %d saying %q, want 3 saying %q", c.name, status, errs, c.flags)
			}
			if got := read(t, filepath.Join(repo, "TASKS.md")); got != before {
				t.Errorf("the refused write changed TASKS.md")
			}
			if got := flagsOf(t, "house-move"); got != c.flags {
				t.Errorf("tasks repos flags %q, want %q", got, c.flags)
			}
		})
	}
}

func TestALineAWriteWouldDropIsRefused(t *testing.T) {
	gitHome(t, map[string]string{"house-move": houseMove + "this line belongs to nothing\n"})
	status, _, errs := run(t, "move", "v9t1", "doing", "-repo", "house-move")
	if status != 3 || !strings.Contains(errs, "would drop it") {
		t.Errorf("a write over a stray line exited %d saying %q, want 3 saying it would drop it", status, errs)
	}
}

func TestTwoTakingOneTaskAtOnceOneIsRefused(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	withRemote(t, filepath.Join(code, "house-move"))

	statuses := make([]int, 2)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Go(func() {
			var out, errs strings.Builder
			statuses[i] = Run([]string{"move", "v9t1", "doing", "-repo", "house-move"}, &out, &errs)
		})
	}
	wg.Wait()
	slices.Sort(statuses)
	if !slices.Equal(statuses, []int{0, 3}) {
		t.Errorf("two takes of v9t1 exited %v, want one 0 and one 3", statuses)
	}
}

func TestAWriteActsOnTheNearestTasksFileAbove(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)
	deep := filepath.Join(repo, "rooms", "kitchen")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deep)
	moving(t, "v9t1", "doing")
	if !strings.Contains(read(t, filepath.Join(repo, "TASKS.md")), "Book the van | doing") {
		t.Error("a move from a folder inside the Repo did not reach its TASKS.md")
	}

	t.Chdir(code)
	if status, _, errs := run(t, "move", "v9t1", "backlog"); status != 1 || !strings.Contains(errs, "-repo") {
		t.Errorf("a move from outside every Repo exited %d saying %q, want 1 pointing at -repo", status, errs)
	}
}

func TestAWriteFromALinkedWorktreeActsOnTheMainCheckout(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	withRemote(t, repo)
	if err := os.MkdirAll(filepath.Join(repo, "rooms"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "rooms", "plan.txt"), []byte("a file the code tracks\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "rooms")
	git(t, repo, "commit", "-q", "-m", "rooms")
	tree := filepath.Join(t.TempDir(), "house-move-wt")
	git(t, repo, "worktree", "add", "-q", tree)

	t.Chdir(filepath.Join(tree, "rooms"))
	moving(t, "v9t1", "doing")
	if !strings.Contains(read(t, filepath.Join(repo, "TASKS.md")), "Book the van | doing") {
		t.Error("a move from a linked worktree did not reach the main checkout's TASKS.md")
	}
	if _, err := os.Stat(filepath.Join(tree, "TASKS.md")); err == nil {
		t.Error("a move from a linked worktree wrote a TASKS.md there")
	}
}

func TestTheActorIsTheEnvironmentThenTheConfigThenTheLogin(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")
	actor := func() string {
		return tasksLog(t, repo, "%(trailers:key=Generated-By,valueonly,separator=)")[0]
	}

	t.Setenv("TASKS_ACTOR", "")
	login, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	moving(t, "v9t1", "doing", "-repo", "house-move")
	if got := actor(); got != login.Username {
		t.Errorf("with no TASKS_ACTOR and no name configured the Actor is %q, want the login %q", got, login.Username)
	}

	config := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "tasks", "config")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("root = ~/code\nname = Neal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	moving(t, "v9t1", "backlog", "-repo", "house-move")
	if got := actor(); got != "Neal" {
		t.Errorf("with a name configured the Actor is %q, want Neal", got)
	}

	t.Setenv("TASKS_ACTOR", "claude-code/claude-opus-5-5")
	moving(t, "v9t1", "doing", "-repo", "house-move")
	if got := actor(); got != "claude-code/claude-opus-5-5" {
		t.Errorf("with TASKS_ACTOR set the Actor is %q, want it", got)
	}
}

func TestAddTakesItsAttributesAndRefusesBadOnes(t *testing.T) {
	code := gitHome(t, map[string]string{"house-move": houseMove})
	repo := filepath.Join(code, "house-move")

	status, out, errs := run(t, "add", "-repo", "house-move", "Pay", "the", "deposit", "-state", "backlog",
		"-priority", "high", "-deadline", "2026-10-15", "-blocked-by", "v9t1", "-why", "the landlord asked")
	if status != 0 {
		t.Fatalf("tasks add exited %d: %s", status, errs)
	}
	id := strings.TrimSpace(out)
	want := "- [ ] Pay the deposit | backlog\n  - id: " + id + "\n  - created: " + today() +
		"\n  - deadline: 2026-10-15\n  - priority: high\n  - why: the landlord asked\n  - blocked by: v9t1\n"
	if file := read(t, filepath.Join(repo, "TASKS.md")); !strings.HasSuffix(file, want) {
		t.Errorf("TASKS.md =\n%s\nwant it to end with\n%s", file, want)
	}

	for _, bad := range [][]string{
		{"add", "-repo", "house-move"},
		{"add", "-repo", "house-move", "Paint", "-priority", "urgent"},
		{"add", "-repo", "house-move", "Paint", "-deadline", "soon"},
		{"add", "-repo", "house-move", "Paint", "-blocked-by", "zzzz"},
		{"add", "-repo", "house-move", "Paint", "-tag", "two words"},
	} {
		if status, _, _ := run(t, bad...); status != 2 {
			t.Errorf("tasks %v exited %d, want 2", bad[1:], status)
		}
	}
}
