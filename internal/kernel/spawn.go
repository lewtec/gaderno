package kernel

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Size advertised to the kernel and its children (COLUMNS / LINES).
// There is no PTY; TUIs that honor the env (nom, workspaced, tqdm) use this.
const (
	kernelTermCols = 80
	kernelTermRows = 24
)

// ExpandArgv replaces Jupyter placeholders in kernelspec argv.
func ExpandArgv(argv []string, connectionFile, resourceDir string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		a = strings.ReplaceAll(a, "{connection_file}", connectionFile)
		a = strings.ReplaceAll(a, "{resource_dir}", resourceDir)
		out[i] = a
	}
	return out
}

// ResolveWorkDir returns an absolute kernel cwd. Empty input is rejected so
// the child never inherits gaderno's process working directory.
func ResolveWorkDir(cwd string) (string, error) {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return "", ErrEmptyWorkDir
	}
	return filepath.Abs(cwd)
}

// StartProcess starts the kernel process with cwd and optional env from spec.
func StartProcess(spec Spec, connectionFile, cwd string) (*exec.Cmd, error) {
	cmd, err := prepareProcess(spec, connectionFile, cwd)
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func prepareProcess(spec Spec, connectionFile, cwd string) (*exec.Cmd, error) {
	argv := ExpandArgv(spec.Spec.Argv, connectionFile, spec.ResourceDir)
	if len(argv) == 0 {
		return nil, ErrEmptyArgv
	}
	dir, err := ResolveWorkDir(cwd)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("COLUMNS=%d", kernelTermCols),
		fmt.Sprintf("LINES=%d", kernelTermRows),
	)
	for k, v := range spec.Spec.Env {
		cmd.Env = append(cmd.Env, k+"="+os.ExpandEnv(v))
	}
	setProcessGroup(cmd)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	return cmd, nil
}
