package kernel

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	uvOnce   sync.Once
	uvSpecs  []Spec
	uvLoaded bool
	// processRoot is the process-lifetime base for catalog work (not a request).
	processRoot = context.Background()
)

// uvListTimeout bounds `uv python list` during catalog load. Catalog is built
// under sync.Once; an unbounded hang would freeze every LoadCatalog caller for
// the process lifetime (kernel chooser, bind checks, metadata resolution).
var uvListTimeout = 15 * time.Second

func resetUVCache() {
	uvOnce = sync.Once{}
	uvSpecs = nil
	uvLoaded = false
}

// listUVSynthetics returns in-memory kernelspecs from `uv python list`.
// Empty if uv is missing or listing fails.
func listUVSynthetics() []Spec {
	uvOnce.Do(func() {
		uvSpecs = loadUVSynthetics()
		uvLoaded = true
	})
	return uvSpecs
}

func loadUVSynthetics() []Spec {
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(processRoot, uvListTimeout)
	defer cancel()
	// Plain Command (not CommandContext): we own the timeout so we can SIGKILL
	// the whole process group. CommandContext only signals the leader and can
	// leave shell grandchildren (or uv helpers) running after the deadline.
	cmd := exec.Command(uvPath, "python", "list")
	setProcessGroup(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case err := <-waitDone:
		if err != nil {
			// Non-zero exit / signal → no uv synthetics (Jupyter kernelspecs
			// still load). Once still completes so the process recovers with an
			// empty uv group rather than hanging forever.
			return nil
		}
	case <-ctx.Done():
		if err := killProcessGroup(cmd, syscall.SIGKILL); err != nil {
			// best-effort kill after uv list timeout
		}
		<-waitDone
		return nil
	}

	keys := parseUVPythonList(stdout.String())
	if len(keys) == 0 {
		return nil
	}

	uvDir := filepath.Dir(uvPath)
	var out []Spec
	seenName := map[string]bool{}
	for _, key := range keys {
		name := uvKernelName(key)
		if name == "" || seenName[name] {
			continue
		}
		seenName[name] = true
		request := uvPythonRequest(key)
		out = append(out, Spec{
			Name:        name,
			ResourceDir: "", // synthetic
			Spec: SpecFile{
				Argv: []string{
					uvPath, "run",
					"--python", request,
					"--with", "ipykernel",
					"--with", "pyzmq",
					"--no-project",
					"--isolated",
					"--refresh",
					"python", "-m", "ipykernel_launcher",
					"-f", "{connection_file}",
				},
				DisplayName: "uv · " + key,
				Language:    "python",
				Env: map[string]string{
					"PATH": uvDir + string(os.PathListSeparator) + "${PATH}",
				},
			},
		})
	}
	return out
}

// parseUVPythonList returns unique first-column keys from `uv python list` text.
func parseUVPythonList(text string) []string {
	var keys []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// first field
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		key := fields[0]
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

// uvKernelName maps a uv list key to a portable kernelspec name.
// e.g. cpython-3.13.7-linux-x86_64-gnu → uv-cpython-3.13.7
// cpython-3.14.6+freethreaded-linux-x86_64-gnu → uv-cpython-3.14.6-freethreaded
var uvKeyRE = regexp.MustCompile(`^([a-z0-9]+)-([0-9]+(?:\.[0-9]+){1,3})(\+([a-z0-9]+))?`)

func uvKernelName(key string) string {
	// strip platform suffix after last version/variant segment
	// keys look like: impl-version[-platform] or impl-version+variant-platform
	m := uvKeyRE.FindStringSubmatch(key)
	if m == nil {
		// fallback: sanitize full key
		safe := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
				return r
			}
			if r == '+' {
				return '-'
			}
			return -1
		}, strings.ToLower(key))
		if safe == "" {
			return ""
		}
		return "uv-" + safe
	}
	impl, ver, variant := m[1], m[2], m[4]
	name := fmt.Sprintf("uv-%s-%s", impl, ver)
	if variant != "" {
		name += "-" + variant
	}
	return name
}

// uvPythonRequest picks a --python argument uv accepts for this list key.
func uvPythonRequest(key string) string {
	// Full list key is accepted by uv for installed and downloadable rows.
	return key
}
