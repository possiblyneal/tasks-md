package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// starter is the tasks.md the template ships, read from the file the template
// copies so the two cannot drift.
func starter(t *testing.T) string {
	t.Helper()
	return read(t, filepath.Join("..", "..", "assets", "starter.md"))
}

// The ignore entries the template ships, exactly as a write would append them.
const templateIgnores = "/tasks.md\n/.tasks.git\n"

func TestARepoMadeFromTheTemplateLintsCleanAndKeepsItsCodeHistoryClean(t *testing.T) {
	text := starter(t)
	code := gitHome(t, map[string]string{"fresh": text})
	repo := filepath.Join(code, "fresh")
	withRemote(t, repo)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(templateIgnores), 0o644); err != nil {
		t.Fatal(err)
	}
	// Generating a repo commits everything the template copied; the ignore
	// entries are what keep the starter out of that commit.
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "chore: generate from the template")
	if tracked := git(t, repo, "ls-files"); tracked != ".gitignore" {
		t.Fatalf("the generating commit tracks %q, want .gitignore alone", tracked)
	}

	if status, out, errs := run(t, "lint", filepath.Join(repo, "tasks.md")); status != 0 || out != "" || errs != "" {
		t.Fatalf("tasks lint on the starter exited %d with %q %q, want 0 and nothing", status, out, errs)
	}
	if status, _, errs := run(t, "add", "Write the first Task", "-repo", "fresh"); status != 0 {
		t.Fatalf("tasks add exited %d: %s", status, errs)
	}

	if file := read(t, filepath.Join(repo, "tasks.md")); !strings.HasPrefix(file, text) {
		t.Errorf("tasks.md =\n%s\nwant it to keep the starter's preamble\n%s", file, text)
	}
	if got := read(t, filepath.Join(repo, ".gitignore")); got != templateIgnores {
		t.Errorf(".gitignore = %q, want the template's entries untouched", got)
	}
	if status := git(t, repo, "status", "--porcelain"); status != "" {
		t.Errorf("the code history sees %q after a write, want nothing", status)
	}
	if status, out, _ := run(t, "lint", filepath.Join(repo, "tasks.md")); status != 0 {
		t.Errorf("tasks lint after the first write exited %d: %s", status, out)
	}
}
