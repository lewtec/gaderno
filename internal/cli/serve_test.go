package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lewtec/lewkit/x/release"
	"github.com/lewtec/lewkit/x/test"
)

func TestResolveServeRoot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		positional string
		flagOrEnv  string
		want       string
	}{
		{name: "positional wins", positional: "/proj", flagOrEnv: "/other", want: "/proj"},
		{name: "flag when no positional", positional: "", flagOrEnv: "/from-flag", want: "/from-flag"},
		{name: "default dot", positional: "", flagOrEnv: "", want: "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveServeRoot(tt.positional, tt.flagOrEnv)
			if got != tt.want {
				t.Fatalf("resolveServeRoot(%q, %q) = %q, want %q", tt.positional, tt.flagOrEnv, got, tt.want)
			}
		})
	}
}

func clearServeEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"GADERNO_ROOT", "GADERNO_LISTEN", "GADERNO_TOKEN", "GADERNO_KERNEL", "GADERNO_I_UNDERSTAND"} {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
}

func parseServe(t *testing.T, args ...string) (serveCmd, error) {
	t.Helper()
	app, err := cmd.Parse[cmd.App[root]](append([]string{"serve"}, args...)...)
	if err != nil {
		return serveCmd{}, err
	}
	if app.Args.Serve == nil {
		t.Fatal("serve command not selected")
	}
	return *app.Args.Serve, nil
}

func TestServeParseDefaults(t *testing.T) {
	clearServeEnv(t)
	got, err := parseServe(t)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root.Value() != "." {
		t.Fatalf("root = %q, want .", got.Root.Value())
	}
	if got.Listen.Value() != "127.0.0.1:8080" {
		t.Fatalf("listen = %q, want 127.0.0.1:8080", got.Listen.Value())
	}
	if got.Token.Value() != "" {
		t.Fatalf("token = %q, want empty", got.Token.Value())
	}
	if got.Kernel.Value() != "python3" {
		t.Fatalf("kernel = %q, want python3", got.Kernel.Value())
	}
	if got.IUnderstand.Value() {
		t.Fatal("i-understand defaulted true")
	}
	if got.Dir != nil {
		t.Fatalf("positional dir = %q, want nil", got.Dir.Value())
	}
}

func TestServeParseFlags(t *testing.T) {
	clearServeEnv(t)
	dir := t.TempDir()
	got, err := parseServe(t,
		"--root", dir,
		"--listen", "127.0.0.1:8765",
		"--token", "secret",
		"--kernel", "uv-cpython-3.13.7",
		"--i-understand",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root.Value() != dir {
		t.Fatalf("root = %q, want %q", got.Root.Value(), dir)
	}
	if got.Listen.Value() != "127.0.0.1:8765" {
		t.Fatalf("listen = %q, want 127.0.0.1:8765", got.Listen.Value())
	}
	if got.Token.Value() != "secret" {
		t.Fatalf("token = %q, want secret", got.Token.Value())
	}
	if got.Kernel.Value() != "uv-cpython-3.13.7" {
		t.Fatalf("kernel = %q, want uv-cpython-3.13.7", got.Kernel.Value())
	}
	if !got.IUnderstand.Value() {
		t.Fatal("i-understand not set")
	}
}

func TestServeParseEnv(t *testing.T) {
	clearServeEnv(t)
	dir := t.TempDir()
	t.Setenv("GADERNO_ROOT", dir)
	t.Setenv("GADERNO_LISTEN", "127.0.0.1:9000")
	t.Setenv("GADERNO_TOKEN", "from-env")
	t.Setenv("GADERNO_KERNEL", "python3")
	t.Setenv("GADERNO_I_UNDERSTAND", "true")
	got, err := parseServe(t)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root.Value() != dir {
		t.Fatalf("root = %q, want %q", got.Root.Value(), dir)
	}
	if got.Listen.Value() != "127.0.0.1:9000" {
		t.Fatalf("listen = %q, want 127.0.0.1:9000", got.Listen.Value())
	}
	if got.Token.Value() != "from-env" {
		t.Fatalf("token = %q, want from-env", got.Token.Value())
	}
	if !got.IUnderstand.Value() {
		t.Fatal("i-understand env not applied")
	}
}

func TestServeFlagOverridesEnv(t *testing.T) {
	clearServeEnv(t)
	t.Setenv("GADERNO_LISTEN", "127.0.0.1:1111")
	t.Setenv("GADERNO_TOKEN", "env-token")
	got, err := parseServe(t, "--listen", "127.0.0.1:2222", "--token", "flag-token")
	if err != nil {
		t.Fatal(err)
	}
	if got.Listen.Value() != "127.0.0.1:2222" {
		t.Fatalf("listen = %q, want flag value", got.Listen.Value())
	}
	if got.Token.Value() != "flag-token" {
		t.Fatalf("token = %q, want flag value", got.Token.Value())
	}
}

func TestServePositionalWinsOverRoot(t *testing.T) {
	clearServeEnv(t)
	pos := t.TempDir()
	flagDir := t.TempDir()
	got, err := parseServe(t, pos, "--root", flagDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dir == nil {
		t.Fatal("positional dir not set")
	}
	if got.Dir.Value() != pos {
		t.Fatalf("positional = %q, want %q", got.Dir.Value(), pos)
	}
	if got.Root.Value() != flagDir {
		t.Fatalf("root = %q, want %q", got.Root.Value(), flagDir)
	}
	if resolved := resolveServeRoot(got.Dir.Value(), got.Root.Value()); resolved != pos {
		t.Fatalf("resolved root = %q, want %q", resolved, pos)
	}
}

func TestServeBarePortListen(t *testing.T) {
	clearServeEnv(t)
	got, err := parseServe(t, "--listen", "8080")
	if err != nil {
		t.Fatal(err)
	}
	if got.Listen.Value() != ":8080" {
		t.Fatalf("listen = %q, want :8080", got.Listen.Value())
	}
}

func TestServeRejectsMissingRoot(t *testing.T) {
	clearServeEnv(t)
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := parseServe(t, "--root", missing)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestServeRejectsExtraArgs(t *testing.T) {
	clearServeEnv(t)
	dir := t.TempDir()
	_, err := parseServe(t, dir, dir)
	if !errors.Is(err, cmd.ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
}

func TestAppUsage(t *testing.T) {
	text, err := cmd.Usage[cmd.App[root]]("gaderno")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"serve",
		"version",
		"--verbose",
		"--version",
		"--help",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage missing %q\n%s", want, text)
		}
	}
}

func TestServeUsage(t *testing.T) {
	text, err := cmd.Usage[serveCmd]("gaderno serve")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"--listen",
		"--root",
		"--token",
		"--kernel",
		"--i-understand",
		"GADERNO_LISTEN",
		"GADERNO_ROOT",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage missing %q\n%s", want, text)
		}
	}
}

func TestVersionCmd(t *testing.T) {
	test.RestoreSlog(t)
	app, err := cmd.Parse[cmd.App[root]]("version")
	if err != nil {
		t.Fatal(err)
	}
	got := test.Stdout(t, func() {
		if err := app.Run(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
	want := release.Version() + "\n"
	if got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
}

func TestVersionFlag(t *testing.T) {
	app, err := cmd.Parse[cmd.App[root]]("--version")
	if err != nil {
		t.Fatal(err)
	}
	got := test.Stdout(t, func() {
		if err := app.Run(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
	want := release.Version() + "\n"
	if got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
}
