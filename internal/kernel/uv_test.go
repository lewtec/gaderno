package kernel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseUVPythonListDedupe(t *testing.T) {
	text := `
cpython-3.13.7-linux-x86_64-gnu                   /path/a
cpython-3.13.7-linux-x86_64-gnu                   /path/b
cpython-3.12.12-linux-x86_64-gnu                  <download available>
pypy-3.11.15-linux-x86_64-gnu                     <download available>
`
	keys := parseUVPythonList(text)
	if len(keys) != 3 {
		t.Fatalf("keys %v", keys)
	}
	if keys[0] != "cpython-3.13.7-linux-x86_64-gnu" {
		t.Fatal(keys[0])
	}
}

func TestUVKernelName(t *testing.T) {
	cases := map[string]string{
		"cpython-3.13.7-linux-x86_64-gnu":              "uv-cpython-3.13.7",
		"cpython-3.14.6+freethreaded-linux-x86_64-gnu": "uv-cpython-3.14.6-freethreaded",
		"pypy-3.11.15-linux-x86_64-gnu":                "uv-pypy-3.11.15",
	}
	for in, want := range cases {
		if got := uvKernelName(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

// A hung `uv python list` must not block catalog load indefinitely.
func TestLoadUVSyntheticsTimesOut(t *testing.T) {
	dir := t.TempDir()
	// Fake uv that ignores args and sleeps longer than the timeout.
	// Use a shell that stays the process leader (no exec) so we exercise
	// process-group kill of the sleep grandchild.
	fake := filepath.Join(dir, "uv")
	script := "#!/bin/sh\nsleep 60\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	old := uvListTimeout
	uvListTimeout = 200 * time.Millisecond
	t.Cleanup(func() { uvListTimeout = old })

	start := time.Now()
	specs := loadUVSynthetics()
	elapsed := time.Since(start)
	if len(specs) != 0 {
		t.Fatalf("want empty on timeout, got %d specs", len(specs))
	}
	if elapsed > 5*time.Second {
		t.Fatalf("loadUVSynthetics took %v; expected to return near timeout", elapsed)
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("loadUVSynthetics returned too fast (%v); fake uv should have been waited on", elapsed)
	}
}
