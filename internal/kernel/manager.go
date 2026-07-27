package kernel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// Manager owns one kernel process + ZMQ connection.
type Manager struct {
	Spec     Spec
	CF       ConnectionFile
	Conn     *Conn
	Cmd      *exec.Cmd
	Session  string
	WorkDir  string
	connPath string
	tmpDir   string
	cancel   context.CancelFunc
	// shellMu serializes shell request/response cycles (execute vs complete).
	shellMu sync.Mutex
}

// Start discovers the kernelspec, writes connection file, starts the process,
// dials ZMQ with long retries, and waits for kernel_info.
func Start(ctx context.Context, kernelName, workDir string) (*Manager, error) {
	spec, err := Find(kernelName)
	if err != nil {
		return nil, err
	}
	cf, err := NewConnectionFile(kernelName)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "gaderno-kernel-*")
	if err != nil {
		return nil, err
	}
	connPath, err := WriteConnectionFile(tmp, cf)
	if err != nil {
		if err := os.RemoveAll(tmp); err != nil {
			// best-effort
		}
		return nil, err
	}
	session := uuid.NewString()

	cmd, err := StartProcess(spec, connPath, workDir)
	if err != nil {
		if err := os.RemoveAll(tmp); err != nil {
			// best-effort
		}
		return nil, err
	}

	// Socket lifetime context — outlives dial attempt timeouts; not cancelled
	// when Start's ctx deadline fires after a successful dial (caller owns Manager).
	sockCtx, sockCancel := context.WithCancel(context.WithoutCancel(ctx))

	var conn *Conn
	deadline := time.Now().Add(2 * time.Minute)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			sockCancel()
			if err := cmd.Process.Kill(); err != nil {
				// best-effort
			}
			if err := cmd.Wait(); err != nil {
				// best-effort
			}
			if err := os.RemoveAll(tmp); err != nil {
				// best-effort
			}
			return nil, err
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			sockCancel()
			if err := os.RemoveAll(tmp); err != nil {
				// best-effort
			}
			return nil, fmt.Errorf("%w: %v", ErrKernelExitedEarly, cmd.ProcessState)
		}
		conn, lastErr = Dial(sockCtx, cf, session)
		if lastErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			sockCancel()
			if err := cmd.Process.Kill(); err != nil {
				// best-effort
			}
			if err := cmd.Wait(); err != nil {
				// best-effort
			}
			if err := os.RemoveAll(tmp); err != nil {
				// best-effort
			}
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	if conn == nil {
		sockCancel()
		if err := cmd.Process.Kill(); err != nil {
			// best-effort
		}
		if err := cmd.Wait(); err != nil {
			// best-effort
		}
		if err := os.RemoveAll(tmp); err != nil {
			// best-effort
		}
		if lastErr == nil {
			lastErr = ErrDialTimeout
		}
		return nil, lastErr
	}

	m := &Manager{
		Spec:     spec,
		CF:       cf,
		Conn:     conn,
		Cmd:      cmd,
		Session:  session,
		WorkDir:  workDir,
		connPath: connPath,
		tmpDir:   tmp,
		cancel:   sockCancel,
	}
	infoCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if _, err := conn.KernelInfo(infoCtx); err != nil {
		if shErr := m.Shutdown(context.WithoutCancel(ctx)); shErr != nil {
			// best-effort cleanup after failed kernel_info
		}
		return nil, fmt.Errorf("kernel_info: %w", err)
	}
	return m, nil
}

// Interrupt asks the kernel to stop the current execution via control-channel
// interrupt_request (Jupyter protocol). Best-effort: returns nil after the
// message is sent; does not wait for the kernel to become idle.
func (m *Manager) Interrupt(ctx context.Context) error {
	if m.Conn == nil {
		return ErrNoConnection
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	req := Message{
		Header:  NewHeader(m.Session, "interrupt_request"),
		Content: map[string]any{},
	}
	return m.Conn.SendControl(req)
}

// Shutdown interrupts and kills the kernel, closes sockets.
func (m *Manager) Shutdown(ctx context.Context) error {
	if m.Conn != nil {
		if err := m.Conn.Close(); err != nil {
			// best-effort
		}
		m.Conn = nil
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.Cmd != nil && m.Cmd.Process != nil {
		if err := killProcessGroup(m.Cmd, syscall.SIGINT); err != nil {
			// best-effort
		}
		done := make(chan error, 1)
		go func() { done <- m.Cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			if err := killProcessGroup(m.Cmd, syscall.SIGKILL); err != nil {
				// best-effort
			}
			<-done
		case <-ctx.Done():
			if err := killProcessGroup(m.Cmd, syscall.SIGKILL); err != nil {
				// best-effort
			}
			<-done
		}
		m.Cmd = nil
	}
	if m.tmpDir != "" {
		if err := os.RemoveAll(m.tmpDir); err != nil {
			// best-effort
		}
	}
	return nil
}

// ConnectionPath returns the connection file path (for tests).
func (m *Manager) ConnectionPath() string {
	return filepath.Clean(m.connPath)
}
