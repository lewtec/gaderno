package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lewtec/lewkit/x/release"
	"github.com/lewtec/lewkit/x/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, tt.want, resolveServeRoot(tt.positional, tt.flagOrEnv))
		})
	}
}

func parseServe(t *testing.T, args ...string) serveCmd {
	t.Helper()
	app, err := cmd.Parse[cmd.App[root]](append([]string{"serve"}, args...)...)
	require.NoError(t, err)
	require.NotNil(t, app.Args.Serve)
	return *app.Args.Serve
}

func TestServeParseDefaults(t *testing.T) {
	got := parseServe(t)
	assert.Equal(t, ".", got.Root.Value())
	assert.Equal(t, "127.0.0.1:8080", got.Listen.Value())
	assert.Empty(t, got.Token.Value())
	assert.Equal(t, "python3", got.Kernel.Value())
	assert.False(t, got.IUnderstand.Value())
	assert.Nil(t, got.Dir)
}

func TestServeParseFlags(t *testing.T) {
	dir := t.TempDir()
	got := parseServe(t,
		"--root", dir,
		"--listen", "127.0.0.1:8765",
		"--token", "secret",
		"--kernel", "uv-cpython-3.13.7",
		"--i-understand",
	)
	assert.Equal(t, dir, got.Root.Value())
	assert.Equal(t, "127.0.0.1:8765", got.Listen.Value())
	assert.Equal(t, "secret", got.Token.Value())
	assert.Equal(t, "uv-cpython-3.13.7", got.Kernel.Value())
	assert.True(t, got.IUnderstand.Value())
}

func TestServeParseEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GADERNO_ROOT", dir)
	t.Setenv("GADERNO_LISTEN", "127.0.0.1:9000")
	t.Setenv("GADERNO_TOKEN", "from-env")
	t.Setenv("GADERNO_KERNEL", "python3")
	t.Setenv("GADERNO_I_UNDERSTAND", "true")
	got := parseServe(t)
	assert.Equal(t, dir, got.Root.Value())
	assert.Equal(t, "127.0.0.1:9000", got.Listen.Value())
	assert.Equal(t, "from-env", got.Token.Value())
	assert.True(t, got.IUnderstand.Value())
}

func TestServePortEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	got := parseServe(t)
	assert.Equal(t, ":9999", got.Listen.Value())
}

func TestServeListenEnvBeatsPort(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("GADERNO_LISTEN", "127.0.0.1:9000")
	got := parseServe(t)
	assert.Equal(t, "127.0.0.1:9000", got.Listen.Value())
}

func TestServeFlagOverridesEnv(t *testing.T) {
	t.Setenv("GADERNO_LISTEN", "127.0.0.1:1111")
	t.Setenv("GADERNO_TOKEN", "env-token")
	got := parseServe(t, "--listen", "127.0.0.1:2222", "--token", "flag-token")
	assert.Equal(t, "127.0.0.1:2222", got.Listen.Value())
	assert.Equal(t, "flag-token", got.Token.Value())
}

func TestServePositionalWinsOverRoot(t *testing.T) {
	pos := t.TempDir()
	flagDir := t.TempDir()
	got := parseServe(t, pos, "--root", flagDir)
	require.NotNil(t, got.Dir)
	assert.Equal(t, pos, got.Dir.Value())
	assert.Equal(t, flagDir, got.Root.Value())
	assert.Equal(t, pos, resolveServeRoot(got.Dir.Value(), got.Root.Value()))
}

func TestServeBarePortListen(t *testing.T) {
	got := parseServe(t, "--listen", "8080")
	assert.Equal(t, ":8080", got.Listen.Value())
}

func TestServeRejectsMissingRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := cmd.Parse[cmd.App[root]]("serve", "--root", missing)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestServeRejectsExtraArgs(t *testing.T) {
	dir := t.TempDir()
	_, err := cmd.Parse[cmd.App[root]]("serve", dir, dir)
	assert.ErrorIs(t, err, cmd.ErrInvalidArgument)
}

func TestAppUsage(t *testing.T) {
	text, err := cmd.Usage[cmd.App[root]]("gaderno")
	require.NoError(t, err)
	for _, want := range []string{"serve", "version", "--verbose", "--version", "--help"} {
		assert.Contains(t, text, want)
	}
}

func TestServeUsage(t *testing.T) {
	text, err := cmd.Usage[serveCmd]("gaderno serve")
	require.NoError(t, err)
	for _, want := range []string{
		"--listen",
		"--root",
		"--token",
		"--kernel",
		"--i-understand",
		"GADERNO_LISTEN",
		"PORT",
		"GADERNO_ROOT",
	} {
		assert.Contains(t, text, want)
	}
}

func TestVersionCmd(t *testing.T) {
	test.RestoreSlog(t)
	app, err := cmd.Parse[cmd.App[root]]("version")
	require.NoError(t, err)
	got := test.Stdout(t, func() {
		require.NoError(t, app.Run(t.Context()))
	})
	assert.Equal(t, release.Version()+"\n", got)
}

func TestVersionFlag(t *testing.T) {
	app, err := cmd.Parse[cmd.App[root]]("--version")
	require.NoError(t, err)
	got := test.Stdout(t, func() {
		require.NoError(t, app.Run(t.Context()))
	})
	assert.Equal(t, release.Version()+"\n", got)
}
