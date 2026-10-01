// Package repos finds the Repos the tracker reads: the folders directly inside
// a configured root that hold a TASKS.md.
//
// The roots come from ConfigPath, a plain key file:
//
//	# one root a line; ~ is the home directory
//	root = ~/code
//	root = ~/house-move
//	# the Actor this host's writes are made as
//	name = neal
//
// No file, or a file naming no root, means one root, ~/code. Blank lines and
// lines starting with # are ignored, and any other key is an error.
package repos

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Repo is a folder holding a TASKS.md. Name is the folder's name under its
// root, which is how a person names it; Path is where it really is, with any
// link resolved.
type Repo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// File is the Repo's TASKS.md.
func (r Repo) File() string { return filepath.Join(r.Path, "TASKS.md") }

// ConfigPath is ~/.config/tasks/config, or under $XDG_CONFIG_HOME when that is
// set.
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tasks", "config"), nil
}

// Config is what the config file says: the roots Repos are found under, and
// the name writes from this host are made as, "" when it gives none.
type Config struct {
	Roots []string
	Name  string
}

// Load reads the config file at path.
func Load(path string) (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	c := Config{Roots: []string{filepath.Join(home, "code")}}

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var roots []string
	scanner := bufio.NewScanner(file)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if ok && key == "name" && value != "" {
			c.Name = value
			continue
		}
		if !ok || key != "root" {
			return Config{}, fmt.Errorf("%s line %d: want `root = <path>` or `name = <actor>`", path, n)
		}
		if rest, ok := strings.CutPrefix(value, "~"); ok {
			value = home + rest
		}
		if !filepath.IsAbs(value) {
			return Config{}, fmt.Errorf("%s line %d: %q is not an absolute path or one under ~", path, n, value)
		}
		roots = append(roots, filepath.Clean(value))
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	if len(roots) > 0 {
		c.Roots = roots
	}
	return c, nil
}

// Find scans the roots afresh: each root's direct children that hold a
// TASKS.md, dot folders skipped and links followed, with two paths to one
// folder counted once. One name under two roots is an error naming both,
// and the first is kept. The Repos come back sorted by name.
func Find(roots []string) ([]Repo, []error) {
	var found []Repo
	var errs []error
	seen := map[string]bool{}
	named := map[string]string{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			errs = append(errs, fmt.Errorf("root %s cannot be read: %w", root, err))
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			path := filepath.Join(root, name)
			if info, err := os.Stat(filepath.Join(path, "TASKS.md")); err != nil || !info.Mode().IsRegular() {
				continue
			}
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if seen[real] {
				continue
			}
			seen[real] = true
			if first, ok := named[name]; ok {
				errs = append(errs, fmt.Errorf("two Repos are named %s: %s and %s", name, first, path))
				continue
			}
			named[name] = path
			found = append(found, Repo{Name: name, Path: real})
		}
	}
	slices.SortFunc(found, func(a, b Repo) int { return strings.Compare(a.Name, b.Name) })
	return found, errs
}
