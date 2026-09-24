package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/gaderno/internal/document"
	"github.com/lucasew/gaderno/internal/store"
)

// Workspace lists and creates notebooks under a rooted directory.
type Workspace struct {
	root string
	st   *store.Store
}

// New creates a workspace rooted at root.
func New(root string) *Workspace {
	return &Workspace{root: root, st: store.New(root)}
}

// List returns relative paths of *.ipynb files under the root (non-recursive for v1).
func (w *Workspace) List() ([]string, error) {
	entries, err := os.ReadDir(w.root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".ipynb") {
			out = append(out, name)
		}
	}
	return out, nil
}

// Create writes a new empty notebook. name may omit .ipynb.
func (w *Workspace) Create(name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".ipynb") {
		name += ".ipynb"
	}
	nb := document.NewEmpty()
	if err := w.st.CreateNew(nil, name, nb); err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("already exists: %s", name)
		}
		return "", err
	}
	return name, nil
}
