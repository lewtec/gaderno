package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-zeromq/zmq4"
)

// Conn is a live ZMQ client connection to a Jupyter kernel.
type Conn struct {
	CF      ConnectionFile
	Session string

	shell   zmq4.Socket
	iopub   zmq4.Socket
	stdin   zmq4.Socket
	control zmq4.Socket
	hb      zmq4.Socket

	mu sync.Mutex
}

func dialOpts() []zmq4.Option {
	return []zmq4.Option{
		zmq4.WithDialerRetry(50 * time.Millisecond),
		zmq4.WithDialerTimeout(500 * time.Millisecond),
		zmq4.WithDialerMaxRetries(2),
	}
}

// Dial opens sockets (IOPub first) and dials endpoints.
func Dial(ctx context.Context, cf ConnectionFile, session string) (*Conn, error) {
	c := &Conn{CF: cf, Session: session}
	opts := dialOpts()

	c.iopub = zmq4.NewSub(ctx, opts...)
	if err := c.iopub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("iopub subscribe: %w", err)
	}
	if err := c.iopub.Dial(cf.Endpoint(cf.IOPubPort)); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("iopub dial: %w", err)
	}

	c.shell = zmq4.NewDealer(ctx, opts...)
	if err := c.shell.Dial(cf.Endpoint(cf.ShellPort)); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("shell dial: %w", err)
	}
	c.control = zmq4.NewDealer(ctx, opts...)
	if err := c.control.Dial(cf.Endpoint(cf.ControlPort)); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("control dial: %w", err)
	}
	c.stdin = zmq4.NewDealer(ctx, opts...)
	if err := c.stdin.Dial(cf.Endpoint(cf.StdinPort)); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("stdin dial: %w", err)
	}
	c.hb = zmq4.NewReq(ctx, opts...)
	if err := c.hb.Dial(cf.Endpoint(cf.HBPort)); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("hb dial: %w", err)
	}
	return c, nil
}

// Close closes all sockets.
func (c *Conn) Close() error {
	var first error
	closeOne := func(s zmq4.Socket) {
		if s == nil {
			return
		}
		if err := s.Close(); err != nil && first == nil {
			first = err
		}
	}
	closeOne(c.shell)
	closeOne(c.iopub)
	closeOne(c.stdin)
	closeOne(c.control)
	closeOne(c.hb)
	return first
}

// SendShell sends a signed message on the shell channel.
func (c *Conn) SendShell(msg Message) error {
	return c.send(c.shell, msg)
}

// RecvShell receives one shell message.
func (c *Conn) RecvShell(ctx context.Context) (Message, error) {
	return c.recv(ctx, c.shell)
}

// RecvIOPub receives one iopub message.
func (c *Conn) RecvIOPub(ctx context.Context) (Message, error) {
	return c.recv(ctx, c.iopub)
}

// KernelInfo sends kernel_info_request and waits for kernel_info_reply.
func (c *Conn) KernelInfo(ctx context.Context) (Message, error) {
	req := Message{
		Header:  NewHeader(c.Session, "kernel_info_request"),
		Content: map[string]any{},
	}
	if err := c.SendShell(req); err != nil {
		return Message{}, err
	}
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	for time.Now().Before(deadline) {
		rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		msg, err := c.RecvShell(rctx)
		cancel()
		if err != nil {
			continue
		}
		if msg.Header.MsgType == "kernel_info_reply" {
			return msg, nil
		}
	}
	return Message{}, fmt.Errorf("timeout waiting for kernel_info_reply")
}

func (c *Conn) send(sock zmq4.Socket, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	frames, err := EncodeWire(c.CF.KeyBytes(), msg)
	if err != nil {
		return err
	}
	return sock.SendMulti(zmq4.NewMsgFrom(frames...))
}

func (c *Conn) recv(ctx context.Context, sock zmq4.Socket) (Message, error) {
	_ = ctx
	msg, err := sock.Recv()
	if err != nil {
		return Message{}, err
	}
	return DecodeWire(c.CF.KeyBytes(), msg.Frames)
}
