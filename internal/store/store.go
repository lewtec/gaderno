package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/gaderno/internal/document"
)

// Store loads and saves notebooks as .ipynb under a jail root.
type Store struct {
	root string
}

// New returns a store rooted at root.
func New(root string) *Store {
	return &Store{root: root}
}

// CleanRel returns a canonical jail-relative path (no leading slash, no "." / "..").
// Callers that key caches or hubs by path must use this so equivalent spellings
// ("./a.ipynb", "a.ipynb", "sub/../a.ipynb") share one entry.
func CleanRel(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	// Force absolute-style Clean so ".." segments collapse even when rel is relative.
	rel = filepath.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("empty path")
	}
	// After leading-slash Clean, ".." path segments are gone. Reject any leftover
	// segment (should not happen) without false-positive on names like "foo..bar".
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == ".." {
			return "", fmt.Errorf("path escapes root")
		}
	}
	return rel, nil
}

// Load reads and parses a notebook at relative path.
func (s *Store) Load(_ context.Context, rel string) (*document.Notebook, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return document.Decode(raw)
}

// Save writes notebook atomically. If the file already exists and createOnly
// semantics are needed, callers should check first. Save overwrites.
func (s *Store) Save(_ context.Context, rel string, nb *document.Notebook) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	document.EnsureCellIDs(nb)
	raw, err := document.Encode(nb)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".gaderno-*.ipynb.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, abs)
}

// CreateNew saves only if the path does not exist.
func (s *Store) CreateNew(ctx context.Context, rel string, nb *document.Notebook) error {
	rel, err := CleanRel(rel)
	if err != nil {
		return err
	}
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	return s.Save(ctx, rel, nb)
}

func (s *Store) resolve(rel string) (string, error) {
	rel, err := CleanRel(rel)
	if err != nil {
		return "", err
	}
	root := filepath.Clean(s.root)
	abs := filepath.Join(root, rel)
	// filepath.Rel rejects paths outside root more reliably than HasPrefix
	// (avoids "/tmp/foo" vs "/tmp/foobar" prefix tricks when root is absolute).
	relToRoot, err := filepath.Rel(root, abs)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root")
	}
	if relToRoot == "." {
		return "", fmt.Errorf("empty path")
	}
	return abs, nil
}

// IsNotExist reports whether err is a missing file.
func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || os.IsNotExist(err)
}
