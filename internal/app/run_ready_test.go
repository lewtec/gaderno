package app

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/lucasew/gaderno/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunReadyReportsAddr(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	got := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunReady(ctx, config.Config{
			Root:   t.TempDir(),
			Listen: "127.0.0.1:0",
			Kernel: "python3",
		}, "test", func(addr string) {
			got <- addr
		})
	}()

	select {
	case addr := <-got:
		client := http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://" + addr + "/healthz")
		require.NoError(t, err)
		body, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "ok\n", string(body))
		cancel()
		require.NoError(t, <-errCh)
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for listen")
	}
}
