package cli

import (
	"testing"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseDesktop(t *testing.T, args ...string) desktopCmd {
	t.Helper()
	app, err := cmd.Parse[cmd.App[root]](append([]string{"desktop"}, args...)...)
	require.NoError(t, err)
	require.NotNil(t, app.Args.Desktop)
	return *app.Args.Desktop
}

func TestDesktopParseDefaults(t *testing.T) {
	got := parseDesktop(t)
	assert.Equal(t, ".", got.Root.Value())
	assert.Equal(t, "127.0.0.1:0", got.Listen.Value())
	assert.Empty(t, got.Token.Value())
	assert.Equal(t, "python3", got.Kernel.Value())
}

func TestDesktopParseShortRoot(t *testing.T) {
	dir := t.TempDir()
	got := parseDesktop(t, "-C", dir, "--listen", "127.0.0.1:9", "--token", "secret", "--kernel", "py")
	assert.Equal(t, dir, got.Root.Value())
	assert.Equal(t, "127.0.0.1:9", got.Listen.Value())
	assert.Equal(t, "secret", got.Token.Value())
	assert.Equal(t, "py", got.Kernel.Value())
}

func TestDesktopURL(t *testing.T) {
	got, err := desktopURL("127.0.0.1:8080", "")
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8080", got)

	got, err = desktopURL("[::1]:8080", "a b")
	require.NoError(t, err)
	assert.Equal(t, "http://[::1]:8080?token=a+b", got)
}

func TestDesktopPageQuotesURL(t *testing.T) {
	page, err := desktopPage("127.0.0.1:8080", `say "hi"`)
	require.NoError(t, err)
	assert.Contains(t, page, `location.replace("http://127.0.0.1:8080?token=say+%22hi%22")`)
}

func TestDesktopUsage(t *testing.T) {
	text, err := cmd.Usage[desktopCmd]("gaderno desktop")
	require.NoError(t, err)
	for _, want := range []string{"-C", "--root", "--listen", "--token", "--kernel", "GADERNO_ROOT"} {
		assert.Contains(t, text, want)
	}
}
