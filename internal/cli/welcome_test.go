package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"runtime"

	"github.com/lewtec/lewkit/x/driver"
	"github.com/lewtec/lewkit/x/ui/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppDirEnvSkipsWelcome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GADERNO_ROOT", dir)
	stubWelcome(t, true, func(context.Context) (string, error) {
		require.Fail(t, "welcome opened")
		return "", nil
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestAppDirPick(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	stubWelcome(t, true, func(context.Context) (string, error) {
		return dir, nil
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Equal(t, dir, got)
	recent, err := gui.Recent()
	require.NoError(t, err)
	require.Len(t, recent, 1)
	assert.Equal(t, dir, recent[0].Path)
}

func TestAppDirCancel(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "")
	t.Setenv("LEWKIT_NO_UI", "")
	ready := filepath.Join(t.TempDir(), "ready.url")
	t.Setenv("ELETROCROMO_READY_FILE", ready)
	stubWelcome(t, true, func(context.Context) (string, error) {
		return "", nil
	})
	require.NoError(t, RunHost(t.Context()))
	_, err := os.Stat(ready)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestAppDirCancelPackaged(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "1")
	t.Setenv("LEWKIT_NO_UI", "")
	t.Setenv("LEWKIT_DATA_DIR", t.TempDir())
	ready := filepath.Join(t.TempDir(), "ready.url")
	t.Setenv("ELETROCROMO_READY_FILE", ready)
	stubWelcome(t, true, func(context.Context) (string, error) {
		return "", context.Canceled
	})
	require.NoError(t, RunHost(t.Context()))
	_, err := os.Stat(ready)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestAppDirEmptyPick(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "")
	t.Setenv("LEWKIT_NO_UI", "")
	stubWelcome(t, true, func(context.Context) (string, error) {
		return "", nil
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestAppDirUnavailable(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "")
	t.Setenv("LEWKIT_NO_UI", "")
	stubWelcome(t, true, func(context.Context) (string, error) {
		return "", driver.ErrUnavailable
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Equal(t, ".", got)
}

func TestAppDirUnavailablePackaged(t *testing.T) {
	data := t.TempDir()
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "1")
	t.Setenv("LEWKIT_NO_UI", "")
	t.Setenv("LEWKIT_DATA_DIR", data)
	stubWelcome(t, true, func(context.Context) (string, error) {
		return "", driver.ErrUnavailable
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(data, "notebooks"), got)
}

func TestAppDirNoWindow(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "")
	t.Setenv("LEWKIT_NO_UI", "")
	stubWelcome(t, false, func(context.Context) (string, error) {
		require.Fail(t, "welcome opened")
		return "", nil
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Equal(t, ".", got)
}

func TestAppDirNoWindowPackaged(t *testing.T) {
	data := t.TempDir()
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "1")
	t.Setenv("LEWKIT_NO_UI", "")
	t.Setenv("LEWKIT_DATA_DIR", data)
	stubWelcome(t, false, func(context.Context) (string, error) {
		require.Fail(t, "welcome opened")
		return "", nil
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(data, "notebooks"), got)
}

func TestAppDirOpenError(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	stubWelcome(t, true, func(context.Context) (string, error) {
		return "", os.ErrClosed
	})
	_, err := appDir(t.Context())
	require.ErrorIs(t, err, os.ErrClosed)
}

func TestAppDirContentSkipsRemember(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	config := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(config, []byte("x"), 0o600))
	t.Setenv("XDG_CONFIG_HOME", config)
	stubWelcome(t, true, func(context.Context) (string, error) {
		return "content://tree", nil
	})
	got, err := appDir(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "content://tree", got)
}

func TestFolderApp(t *testing.T) {
	t.Setenv("ELETROCROMO_NO_UI", "")
	t.Setenv("LEWKIT_NO_UI", "")
	assert.True(t, folderApp())
	t.Setenv("ELETROCROMO_NO_UI", "1")
	assert.Equal(t, runtime.GOOS == "android", folderApp())
}

func TestReleaseStampedDev(t *testing.T) {
	assert.False(t, releaseStamped())
}

func TestAppLogo(t *testing.T) {
	img, err := appLogo()
	require.NoError(t, err)
	require.NotNil(t, img)
	assert.Positive(t, img.Bounds().Dx())
	assert.Positive(t, img.Bounds().Dy())
}

func stubWelcome(t *testing.T, show bool, open func(context.Context) (string, error)) {
	t.Helper()
	prevShow, prevOpen := showWelcome, openWelcome
	showWelcome = func(context.Context) bool { return show }
	openWelcome = open
	t.Cleanup(func() {
		showWelcome = prevShow
		openWelcome = prevOpen
	})
}
