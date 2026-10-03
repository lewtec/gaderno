package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostLaunch(t *testing.T) {
	tests := []struct {
		name string
		noUI string
		args []string
		want bool
	}{
		{name: "bare", want: true},
		{name: "help", args: []string{"-h"}, want: false},
		{name: "serve", args: []string{"serve"}, want: false},
		{name: "desktop", args: []string{"desktop"}, want: false},
		{name: "host with args", noUI: "1", args: []string{"serve"}, want: true},
		{name: "lewkit no ui", noUI: "true", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "lewkit no ui" {
				t.Setenv("ELETROCROMO_NO_UI", "")
				t.Setenv("LEWKIT_NO_UI", tt.noUI)
			} else {
				t.Setenv("ELETROCROMO_NO_UI", tt.noUI)
				t.Setenv("LEWKIT_NO_UI", "")
			}
			if got := HostLaunch(tt.args); got != tt.want {
				t.Errorf("HostLaunch(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestHostRootCwd(t *testing.T) {
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "")
	t.Setenv("LEWKIT_NO_UI", "")
	got, err := hostRoot()
	require.NoError(t, err)
	assert.Equal(t, ".", got)
}

func TestHostRootEnvWins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GADERNO_ROOT", dir)
	t.Setenv("ELETROCROMO_NO_UI", "1")
	got, err := hostRoot()
	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestHostRootPackaged(t *testing.T) {
	data := t.TempDir()
	t.Setenv("GADERNO_ROOT", "")
	t.Setenv("ELETROCROMO_NO_UI", "1")
	t.Setenv("LEWKIT_NO_UI", "")
	t.Setenv("LEWKIT_DATA_DIR", data)
	got, err := hostRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(data, "notebooks"), got)
	st, err := os.Stat(got)
	require.NoError(t, err)
	assert.True(t, st.IsDir())
}

func TestHostListen(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("GADERNO_LISTEN", "")
		assert.Equal(t, "127.0.0.1:0", hostListen())
	})
	t.Run("bare port", func(t *testing.T) {
		t.Setenv("GADERNO_LISTEN", "8765")
		assert.Equal(t, "127.0.0.1:8765", hostListen())
	})
	t.Run("host port", func(t *testing.T) {
		t.Setenv("GADERNO_LISTEN", "127.0.0.1:9")
		assert.Equal(t, "127.0.0.1:9", hostListen())
	})
}

func TestAnnounceReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ready.url")
	t.Setenv("ELETROCROMO_READY_FILE", path)
	announceReady("127.0.0.1:9", "s3cret")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:9?token=s3cret\n", string(body))
}

func TestServeReady(t *testing.T) {
	root := t.TempDir()
	ready := filepath.Join(t.TempDir(), "ready.url")
	t.Setenv("GADERNO_ROOT", root)
	t.Setenv("GADERNO_LISTEN", "")
	t.Setenv("GADERNO_TOKEN", "")
	t.Setenv("GADERNO_KERNEL", "python3")
	t.Setenv("ELETROCROMO_NO_UI", "1")
	t.Setenv("LEWKIT_NO_UI", "")
	t.Setenv("ELETROCROMO_READY_FILE", ready)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- RunHost(ctx) }()

	var raw string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(ready)
		if err == nil {
			raw = strings.TrimSpace(string(body))
			if raw != "" {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotEmpty(t, raw)
	require.True(t, strings.HasPrefix(raw, "http://127.0.0.1:"), "ready url %s", raw)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(raw + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok\n", string(body))

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		require.Fail(t, "shutdown")
	}
}
