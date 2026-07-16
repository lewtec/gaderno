package kernel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverJupyterPath(t *testing.T) {
	root := t.TempDir()
	kdir := filepath.Join(root, "kernels", "fakekernel")
	if err := os.MkdirAll(kdir, 0o755); err != nil {
		t.Fatal(err)
	}
	kj := SpecFile{
		Argv:        []string{"echo", "{connection_file}"},
		DisplayName: "Fake",
		Language:    "echo",
	}
	raw, _ := json.Marshal(kj)
	if err := os.WriteFile(filepath.Join(kdir, "kernel.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JUPYTER_PATH", root)
	// clear others that might shadow
	t.Setenv("JUPYTER_DATA_DIR", filepath.Join(t.TempDir(), "empty"))

	all, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range all {
		if s.Name == "fakekernel" {
			found = true
			if s.Spec.DisplayName != "Fake" {
				t.Fatalf("display %q", s.Spec.DisplayName)
			}
		}
	}
	if !found {
		t.Fatalf("fakekernel not found in %#v", all)
	}
}

func TestFindMissing(t *testing.T) {
	t.Setenv("JUPYTER_PATH", t.TempDir())
	t.Setenv("JUPYTER_DATA_DIR", t.TempDir())
	_, err := Find("definitely-missing-kernel-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
}
