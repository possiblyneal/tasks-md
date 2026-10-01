// Package history is a Repo's tasks history (ADR 0005): a second git history
// beside the code's, in `.tasks.git`, tracking TASKS.md alone and pushed after
// every write to a `tasks` branch on the Repo's own remote.
//
// Write is the one way the file changes through the tracker. It takes the
// Repo's lock, readies the history, commits any hand edit first as a direct
// edit, applies the change it was handed, commits the file with the Actor in a
// trailer, and pushes. It never reads inside the file: what a change does to
// the text is its caller's.
//
// Flags says what the board marks a Repo with: not pushed, not backed up, or
// refusing writes and why.
//
// Log and Show read the history back, for a Task's history to be drawn from.
package history

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	gitDir = ".tasks.git"
	file   = "TASKS.md"
	branch = "tasks"
	remote = "origin"
)

// The flags a Repo can carry, beside the refusal's own sentence.
const (
	NotPushed   = "not pushed"
	NotBackedUp = "not backed up"
)

// DirectEdit is the Actor a hand edit is committed as: nobody the tracker can
// name.
const DirectEdit = "direct edit"

// Unready is a Repo refusing writes: its TASKS.md holds conflict markers, or
// its tasks history is mid-merge, mid-rebase or on a detached HEAD. A person
// clears it by hand, and nothing is written until they do.
type Unready struct{ Reason string }

func (u Unready) Error() string { return "refusing writes: " + u.Reason }

// Change is what one write does to the file: the text after it, given the text
// before, and the subject naming it after `chore(tasks): `, such as
// `move ^m3qa to Doing`. A text that comes back unchanged commits nothing.
type Change func(before string) (after, subject string, err error)

// Write applies change to dir's TASKS.md under the Repo's lock and records it
// as one commit authored by the host's git user, carrying the Actor. A push
// that fails does not fail the write: the commit stays, and Flags says the
// Repo is not pushed until a later write's push goes through.
func Write(dir, actor string, change Change) error {
	unlock, err := lock(dir)
	if err != nil {
		return err
	}
	defer unlock()

	if err := ready(dir); err != nil {
		return err
	}
	if err := commit(dir, "commit a direct edit", DirectEdit); err != nil {
		return err
	}
	path := filepath.Join(dir, file)
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	after, subject, err := change(string(before))
	if err != nil {
		return err
	}
	if after == string(before) {
		return nil
	}
	if err := replace(path, after); err != nil {
		return err
	}
	if err := commit(dir, subject, actor); err != nil {
		// The write is undone rather than left for the next one to commit as
		// a direct edit nobody made.
		return errors.Join(err, replace(path, string(before)))
	}
	push(dir)
	return nil
}

// Flags is what the board marks dir with, worked out afresh on every read.
func Flags(dir string) []string {
	flags := []string{}
	if err := unready(dir); err != nil {
		flags = append(flags, err.Error())
	}
	switch {
	case codeRemote(dir) == "":
		flags = append(flags, NotBackedUp)
	case !pushed(dir):
		flags = append(flags, NotPushed)
	}
	return flags
}

// MainCheckout is dir as seen from the main checkout when dir is inside a
// linked git worktree, and dir itself otherwise. A worktree's copy of a Repo's
// folder is not where its Tasks live, so a write run there acts on the main
// checkout's.
func MainCheckout(dir string) string {
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	out, err := run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir", "--git-dir", "--show-toplevel")
	if err != nil {
		return dir
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 3 || lines[0] == lines[1] {
		return dir
	}
	rel, err := filepath.Rel(lines[2], dir)
	if err != nil {
		return dir
	}
	return filepath.Join(filepath.Dir(lines[0]), rel)
}

// Commit is one write in a Repo's tasks history. Subject is what follows
// `chore(tasks): `, Actor the Generated-By trailer, and Parent the commit
// before it, "" for the first.
type Commit struct {
	Hash, Parent string
	At           time.Time
	Actor        string
	Subject      string
}

// Log is dir's tasks history, newest first. A Repo never written through has
// none, and that is no error.
func Log(dir string) ([]Commit, error) {
	if _, err := os.Stat(filepath.Join(dir, gitDir)); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if _, err := tasks(dir, "rev-parse", "-q", "--verify", "HEAD"); err != nil {
		return nil, nil
	}
	// One record a commit, its fields split by NUL and records by RS, so no
	// subject or trailer can be mistaken for a boundary.
	out, err := tasks(dir, "log", "--first-parent",
		"--format=%H%x00%P%x00%aI%x00%s%x00%(trailers:key=Generated-By,valueonly,separator=)%x1e")
	if err != nil {
		return nil, err
	}
	var log []Commit
	for record := range strings.SplitSeq(out, "\x1e") {
		fields := strings.Split(strings.TrimSpace(record), "\x00")
		if len(fields) != 5 {
			continue
		}
		at, err := time.Parse(time.RFC3339, fields[2])
		if err != nil {
			return nil, err
		}
		parent, _, _ := strings.Cut(fields[1], " ")
		log = append(log, Commit{
			Hash:    fields[0],
			Parent:  parent,
			At:      at,
			Subject: strings.TrimPrefix(fields[3], "chore(tasks): "),
			Actor:   strings.TrimSpace(fields[4]),
		})
	}
	return log, nil
}

// Show is TASKS.md as the commit left it, and "" for no commit at all.
func Show(dir, hash string) (string, error) {
	if hash == "" {
		return "", nil
	}
	return tasks(dir, "show", hash+":"+file)
}

// lock holds the Repo's folder for one write, so two writers read, change
// and commit one after the other. The folder is locked rather than the file
// because the file is replaced by a rename, which a lock on it would not
// survive.
func lock(dir string) (func(), error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() { _ = f.Close() }, nil
}

// ready gives a Repo a tasks history where it has none, makes the code's
// history ignore the file and the history, and refuses a Repo in a mess.
func ready(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, gitDir)); errors.Is(err, os.ErrNotExist) {
		if _, err := tasks(dir, "init", "-q", "--initial-branch="+branch); err != nil {
			return err
		}
		// The history sees TASKS.md and nothing else in the folder, so its
		// own status is readable by a person.
		exclude := filepath.Join(dir, gitDir, "info", "exclude")
		if err := os.WriteFile(exclude, []byte("/*\n!/"+file+"\n"), 0o644); err != nil {
			return err
		}
	}
	if err := ignored(dir); err != nil {
		return err
	}
	return unready(dir)
}

// ignored appends TASKS.md and .tasks.git to the Repo's .gitignore where its
// code history does not already ignore them. A folder holding no code has no
// code history to keep them out of.
func ignored(dir string) error {
	if !isCode(dir) {
		return nil
	}
	var missing []string
	for _, name := range []string{file, gitDir} {
		// check-ignore exits 1 for a path no rule ignores.
		if _, err := run(dir, "check-ignore", "-q", "--no-index", name); err != nil {
			missing = append(missing, "/"+name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	path := filepath.Join(dir, ".gitignore")
	text, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(text) > 0 && !bytes.HasSuffix(text, []byte("\n")) {
		text = append(text, '\n')
	}
	text = append(text, strings.Join(missing, "\n")+"\n"...)
	return os.WriteFile(path, text, 0o644)
}

// unready says why dir refuses writes, or nil.
func unready(dir string) error {
	if text, err := os.ReadFile(filepath.Join(dir, file)); err == nil && conflicted(string(text)) {
		return Unready{"TASKS.md holds conflict markers"}
	}
	git := filepath.Join(dir, gitDir)
	for name, reason := range map[string]string{
		"MERGE_HEAD":   "its tasks history is mid-merge",
		"rebase-merge": "its tasks history is mid-rebase",
		"rebase-apply": "its tasks history is mid-rebase",
	} {
		if _, err := os.Stat(filepath.Join(git, name)); err == nil {
			return Unready{reason}
		}
	}
	if head, err := os.ReadFile(filepath.Join(git, "HEAD")); err == nil && !bytes.HasPrefix(head, []byte("ref: ")) {
		return Unready{"its tasks history is on a detached HEAD"}
	}
	return nil
}

func conflicted(text string) bool {
	for line := range strings.Lines(text) {
		for _, marker := range []string{"<<<<<<<", "=======", ">>>>>>>", "|||||||"} {
			if strings.HasPrefix(line, marker) {
				return true
			}
		}
	}
	return false
}

// commit commits TASKS.md alone if it differs from the history's last
// commit, with hooks skipped. Nothing else is ever added to the history's
// index, so committing the index commits the file alone. The add is forced
// because the folder's .gitignore, which this history reads too, ignores the
// file for the code's sake.
func commit(dir, subject, actor string) error {
	if _, err := tasks(dir, "add", "-f", "--", file); err != nil {
		return err
	}
	// diff --quiet exits 1 when the index holds a change, and 0 when not.
	if _, err := tasks(dir, "diff", "--cached", "--quiet"); err == nil {
		return nil
	}
	_, err := tasks(dir, "commit", "-q", "--no-verify",
		"-m", "chore(tasks): "+subject, "-m", "Generated-By: "+actor)
	return err
}

// push sends the history to the `tasks` branch on the code's origin. The
// remote is set from the code's on every push, so a Repo given a remote later
// is backed up from its next write. A failure is left for Flags to report.
func push(dir string) {
	url := codeRemote(dir)
	if url == "" {
		return
	}
	if _, err := tasks(dir, "remote", "get-url", remote); err != nil {
		_, _ = tasks(dir, "remote", "add", "-t", branch, remote, url)
	} else {
		_, _ = tasks(dir, "remote", "set-url", remote, url)
	}
	_, _ = tasks(dir, "push", "-q", remote, "HEAD:refs/heads/"+branch)
}

// pushed says whether the remote's tasks branch, as the last push left it,
// is the history's HEAD. A Repo never written through has nothing to push.
func pushed(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, gitDir)); err != nil {
		return true
	}
	out, err := tasks(dir, "rev-parse", "HEAD", "refs/remotes/"+remote+"/"+branch)
	if err != nil {
		return false
	}
	heads := strings.Fields(out)
	return len(heads) == 2 && heads[0] == heads[1]
}

// codeRemote is the URL of the code history's origin, or "" where the folder
// holds no code or its code has no origin.
func codeRemote(dir string) string {
	if !isCode(dir) {
		return ""
	}
	url, _ := run(dir, "config", "--get", "remote."+remote+".url")
	return url
}

func isCode(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// tasks runs git against dir's tasks history, its worktree the folder.
func tasks(dir string, args ...string) (string, error) {
	return run(dir, append([]string{"--git-dir=" + gitDir, "--work-tree=."}, args...)...)
}

// run runs git in dir with no hooks, no prompt and none of the variables a
// calling git hook would point it elsewhere with.
func run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(scrubbed(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func scrubbed() []string {
	var env []string
	for _, kv := range os.Environ() {
		switch name, _, _ := strings.Cut(kv, "="); name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR",
			"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_PREFIX":
		default:
			env = append(env, kv)
		}
	}
	return env
}

// replace writes the file whole under a new name and renames it into place,
// so a reader never sees half of it.
func replace(path, text string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".TASKS.md.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil {
		_ = os.Chmod(tmp.Name(), info.Mode().Perm())
	}
	return os.Rename(tmp.Name(), path)
}
