package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartKernelInfo(t *testing.T) {
	if os.Getenv("GADERNO_KERNEL_TEST") == "" {
		t.Skip("set GADERNO_KERNEL_TEST=1 to run real kernel test")
	}
	uv := os.Getenv("GADERNO_UV")
	if uv == "" {
		uv = "/home/lucasew/.local/share/mise/installs/uv/0.11.26/bin/uv"
	}
	if uv == "" {
		t.Skip("uv not on PATH")
	}
	if _, err := os.Stat(uv); err != nil {
		t.Skipf("uv not found at %s: %v", uv, err)
	}
	root := t.TempDir()
	kdir := filepath.Join(root, "kernels", "gaderno-test")
	if err := os.MkdirAll(kdir, 0o755); err != nil {
		t.Fatal(err)
	}
	kj := SpecFile{
		Argv: []string{
			uv, "run", "--python", "3.12",
			"--with", "ipykernel", "--with", "pyzmq",
			"--no-project", "--isolated",
			"python", "-m", "ipykernel_launcher", "-f", "{connection_file}",
		},
		DisplayName: "gaderno-test",
		Language:    "python",
	}
	raw, _ := json.Marshal(kj)
	if err := os.WriteFile(filepath.Join(kdir, "kernel.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JUPYTER_PATH", root)
	t.Setenv("JUPYTER_DATA_DIR", filepath.Join(t.TempDir(), "empty"))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	m, err := Start(ctx, "gaderno-test", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	t.Logf("kernel ready pid=%v", m.Cmd.Process.Pid)
}
