package repos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// folder makes dir under root, with a tasks.md when held is true.
func folder(t *testing.T, root, dir string, held bool) string {
	t.Helper()
	path := filepath.Join(root, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if held {
		if err := os.WriteFile(filepath.Join(path, "tasks.md"), []byte("# Tasks\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func names(found []Repo) string {
	var out []string
	for _, r := range found {
		out = append(out, r.Name)
	}
	return strings.Join(out, " ")
}

func TestFindTakesOnlyDirectChildrenHoldingATasksFile(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	folder(t, root, "house-move", true)
	folder(t, root, "work", true)
	folder(t, root, "no-tasks", false)
	folder(t, root, ".hidden", true)
	folder(t, root, "work/nested-clone", true)
	folder(t, root, "work/tmp", true)
	folder(t, root, "no-tasks/tmp/deep", true)

	found, errs := Find([]string{root})
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if got := names(found); got != "house-move work" {
		t.Errorf("repos = %q, want %q", got, "house-move work")
	}
	if found[0].Path != filepath.Join(root, "house-move") {
		t.Errorf("path = %q", found[0].Path)
	}
}

func TestFindFollowsALinkAndCountsItsTargetOnce(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	elsewhere, _ := filepath.EvalSymlinks(t.TempDir())
	target := folder(t, elsewhere, "linked", true)
	if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	folder(t, root, "alpha", true)
	if err := os.Symlink(filepath.Join(root, "alpha"), filepath.Join(root, "alpha-again")); err != nil {
		t.Fatal(err)
	}

	found, errs := Find([]string{root})
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if got := names(found); got != "alpha linked" {
		t.Errorf("repos = %q, want %q", got, "alpha linked")
	}
	if found[1].Path != target {
		t.Errorf("linked path = %q, want %q", found[1].Path, target)
	}
}

func TestOneNameUnderTwoRootsIsAnErrorNamingBoth(t *testing.T) {
	one, _ := filepath.EvalSymlinks(t.TempDir())
	two, _ := filepath.EvalSymlinks(t.TempDir())
	first := folder(t, one, "notes", true)
	second := folder(t, two, "notes", true)

	_, errs := Find([]string{one, two})
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want one", errs)
	}
	if msg := errs[0].Error(); !strings.Contains(msg, first) || !strings.Contains(msg, second) {
		t.Errorf("error = %q, want both %s and %s", msg, first, second)
	}
}

func TestFindSaysWhenARootIsNotThere(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	_, errs := Find([]string{gone})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), gone) {
		t.Errorf("errors = %v, want one naming %s", errs, gone)
	}
}

func TestLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	c, err := Load(path)
	if err != nil || len(c.Roots) != 1 || c.Roots[0] != filepath.Join(home, "code") || c.Name != "" {
		t.Errorf("no config: %+v, %v; want roots [%s] and no name", c, err, filepath.Join(home, "code"))
	}

	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("# where Repos live\nroot = ~/code\n\nroot = /srv/house\nname = neal\n")
	c, err = Load(path)
	if err != nil || strings.Join(c.Roots, " ") != filepath.Join(home, "code")+" /srv/house" || c.Name != "neal" {
		t.Errorf("config = %+v, %v", c, err)
	}

	for _, bad := range []string{"roots: ~/code\n", "colour = red\n", "root = relative/path\n"} {
		write(bad)
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("config %q: err = %v, want one naming line 1", bad, err)
		}
	}
}
