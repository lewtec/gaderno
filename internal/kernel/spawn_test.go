package kernel

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkDirRejectsEmpty(t *testing.T) {
	t.Parallel()
	for _, cwd := range []string{"", "   "} {
		_, err := ResolveWorkDir(cwd)
		if !errors.Is(err, ErrEmptyWorkDir) {
			t.Fatalf("ResolveWorkDir(%q) err = %v, want ErrEmptyWorkDir", cwd, err)
		}
	}
}

func TestPrepareProcessSetsAbsoluteDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd, err := prepareProcess(Spec{Spec: SpecFile{Argv: []string{"kernel"}}}, "conn.json", dir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != want {
		t.Fatalf("Dir = %q, want %q", cmd.Dir, want)
	}
}

func TestPrepareProcessEmptyCwd(t *testing.T) {
	t.Parallel()
	_, err := prepareProcess(Spec{Spec: SpecFile{Argv: []string{"kernel"}}}, "conn.json", "")
	if !errors.Is(err, ErrEmptyWorkDir) {
		t.Fatalf("err = %v, want ErrEmptyWorkDir", err)
	}
}

func TestPrepareProcessRelativeCwdIsAbs(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	cmd, err := prepareProcess(Spec{Spec: SpecFile{Argv: []string{"kernel"}}}, "conn.json", ".")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != want {
		t.Fatalf("Dir = %q, want %q (must not inherit a caller-relative cwd)", cmd.Dir, want)
	}
}

func TestPrepareProcessAnnouncesTermSize(t *testing.T) {
	t.Parallel()
	cmd, err := prepareProcess(Spec{Spec: SpecFile{Argv: []string{"kernel"}}}, "conn.json", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var cols, lines string
	for _, e := range cmd.Env {
		switch {
		case strings.HasPrefix(e, "COLUMNS="):
			cols = e
		case strings.HasPrefix(e, "LINES="):
			lines = e
		}
	}
	if cols != "COLUMNS=80" || lines != "LINES=24" {
		t.Fatalf("term size env COLUMNS=%q LINES=%q", cols, lines)
	}
}

func TestPrepareProcessEmptyArgv(t *testing.T) {
	t.Parallel()
	_, err := prepareProcess(Spec{Spec: SpecFile{}}, "conn.json", t.TempDir())
	if !errors.Is(err, ErrEmptyArgv) {
		t.Fatalf("err = %v, want ErrEmptyArgv", err)
	}
}
