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

	shellCh chan Message
	iopubCh chan Message

	mu     sync.Mutex
	closed bool
	cancel context.CancelFunc
}

func dialOpts() []zmq4.Option {
	return []zmq4.Option{
		zmq4.WithDialerRetry(50 * time.Millisecond),
		zmq4.WithDialerTimeout(500 * time.Millisecond),
		zmq4.WithDialerMaxRetries(2),
	}
}

// Dial opens sockets (IOPub first) and starts reader loops.
func Dial(ctx context.Context, cf ConnectionFile, session string) (*Conn, error) {
	c := &Conn{
		CF:      cf,
		Session: session,
		shellCh: make(chan Message, 64),
		iopubCh: make(chan Message, 256),
	}
	opts := dialOpts()
	// Own cancel for Close; WithoutCancel so dial timeout/cancel does not kill
	// reader loops after a successful dial (and preserves ctx values).
	rctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	c.cancel = cancel

	c.iopub = zmq4.NewSub(ctx, opts...)
	if err := c.iopub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		if err := c.Close(); err != nil {
			// best-effort
		}
		return nil, fmt.Errorf("iopub subscribe: %w", err)
	}
	if err := c.iopub.Dial(cf.Endpoint(cf.IOPubPort)); err != nil {
		if err := c.Close(); err != nil {
			// best-effort
		}
		return nil, fmt.Errorf("iopub dial: %w", err)
	}

	c.shell = zmq4.NewDealer(ctx, opts...)
	if err := c.shell.Dial(cf.Endpoint(cf.ShellPort)); err != nil {
		if err := c.Close(); err != nil {
			// best-effort
		}
		return nil, fmt.Errorf("shell dial: %w", err)
	}
	c.control = zmq4.NewDealer(ctx, opts...)
	if err := c.control.Dial(cf.Endpoint(cf.ControlPort)); err != nil {
		if err := c.Close(); err != nil {
			// best-effort
		}
		return nil, fmt.Errorf("control dial: %w", err)
	}
	c.stdin = zmq4.NewDealer(ctx, opts...)
	if err := c.stdin.Dial(cf.Endpoint(cf.StdinPort)); err != nil {
		if err := c.Close(); err != nil {
			// best-effort
		}
		return nil, fmt.Errorf("stdin dial: %w", err)
	}
	c.hb = zmq4.NewReq(ctx, opts...)
	if err := c.hb.Dial(cf.Endpoint(cf.HBPort)); err != nil {
		if err := c.Close(); err != nil {
			// best-effort
		}
		return nil, fmt.Errorf("hb dial: %w", err)
	}

	go c.readLoop(rctx, c.shell, c.shellCh)
	go c.readLoop(rctx, c.iopub, c.iopubCh)
	return c, nil
}

func (c *Conn) readLoop(ctx context.Context, sock zmq4.Socket, out chan<- Message) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		raw, err := sock.Recv()
		if err != nil {
			return
		}
		msg, err := DecodeWire(c.CF.KeyBytes(), raw.Frames)
		if err != nil {
			continue
		}
		select {
		case out <- msg:
		case <-ctx.Done():
			return
		}
	}
}

// Close closes all sockets and stops readers.
func (c *Conn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()

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

// SendControl sends a signed message on the control channel (interrupt, shutdown).
func (c *Conn) SendControl(msg Message) error {
	return c.send(c.control, msg)
}

// RecvShell receives one shell message (from reader loop).
func (c *Conn) RecvShell(ctx context.Context) (Message, error) {
	return recvMsg(ctx, c.shellCh, ErrShellClosed)
}

// RecvIOPub receives one iopub message (from reader loop).
func (c *Conn) RecvIOPub(ctx context.Context) (Message, error) {
	return recvMsg(ctx, c.iopubCh, ErrIOPubClosed)
}

func recvMsg(ctx context.Context, ch <-chan Message, closed error) (Message, error) {
	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case msg, ok := <-ch:
		if !ok {
			return Message{}, closed
		}
		return msg, nil
	}
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
	for {
		msg, err := c.RecvShell(ctx)
		if err != nil {
			return Message{}, err
		}
		if msg.Header.MsgType == "kernel_info_reply" {
			return msg, nil
		}
	}
}

func (c *Conn) send(sock zmq4.Socket, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrConnectionClosed
	}
	frames, err := EncodeWire(c.CF.KeyBytes(), msg)
	if err == nil {
		err = sock.SendMulti(zmq4.NewMsgFrom(frames...))
	}
	return err
}
