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
	rel = filepath.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("empty path")
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("path escapes root")
	}
	abs := filepath.Join(s.root, rel)
	// Ensure still under root
	root := filepath.Clean(s.root)
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root")
	}
	return abs, nil
}

// IsNotExist reports whether err is a missing file.
func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || os.IsNotExist(err)
}
