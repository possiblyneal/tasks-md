package write

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/possiblyneal/tasks-md/apps/tasks/src/taskfile"
)

// A deleted Task's id stays spent, so a later Task never answers to its
// history.
func TestADeletedTasksIDIsNeverHandedOutAgain(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	config := "[user]\n\tname = Host Person\n\temail = host@example.com\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TASKS.md"), []byte("# Tasks\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := Add(dir, "neal", New{Title: "Gone soon"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(dir, "neal", id, ""); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(dir, "TASKS.md"))
	if err != nil {
		t.Fatal(err)
	}
	f, _ := taskfile.Parse(string(text))
	used, err := spent(dir, f)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := used[id]; !ok {
		t.Errorf("the ids spent = %v, want the deleted ^%s among them", used, id)
	}
}
