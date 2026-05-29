package at

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
)

type Logger func(format string, args ...any)

type Client struct {
	name    string
	port    serial.Port
	timeout time.Duration
	log     Logger
}

type Response struct {
	Command string
	Lines   []string
	Raw     string
	OK      bool
}

func (r Response) Text() string {
	return strings.Join(r.Lines, " | ")
}

func Open(name string, timeout time.Duration, log Logger) (*Client, error) {
	if log == nil {
		log = func(string, ...any) {}
	}
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(name, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial port %s: %w", name, err)
	}
	if err := port.SetReadTimeout(500 * time.Millisecond); err != nil {
		return nil, errors.Join(fmt.Errorf("set read timeout on %s: %w", name, err), port.Close())
	}
	if err := port.SetDTR(true); err != nil {
		return nil, errors.Join(fmt.Errorf("set DTR on %s: %w", name, err), port.Close())
	}
	if err := port.SetRTS(true); err != nil {
		return nil, errors.Join(fmt.Errorf("set RTS on %s: %w", name, err), port.Close())
	}
	return &Client{name: name, port: port, timeout: timeout, log: log}, nil
}

func (c *Client) Name() string { return c.name }

func (c *Client) Close() error { return c.port.Close() }

func (c *Client) Send(command string) (Response, error) {
	c.log("TX %s", command)
	if _, err := c.port.Write([]byte(command + "\r")); err != nil {
		return Response{Command: command}, fmt.Errorf("write %q: %w", command, err)
	}
	var raw strings.Builder
	buf := make([]byte, 512)
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		n, err := c.port.Read(buf)
		if n > 0 {
			raw.Write(buf[:n])
			if done, ok := terminated(raw.String()); done {
				return c.finish(command, raw.String(), ok), nil
			}
		}
		if err != nil {
			return c.finish(command, raw.String(), false), fmt.Errorf("read after %q: %w", command, err)
		}
	}
	return c.finish(command, raw.String(), false), fmt.Errorf("timeout waiting for response to %q", command)
}

func (c *Client) SendAll(commands []string, pause time.Duration) {
	for _, cmd := range commands {
		if _, err := c.Send(cmd); err != nil {
			c.log("send %s: %v", cmd, err)
		}
		time.Sleep(pause)
	}
}

func (c *Client) finish(command, raw string, ok bool) Response {
	lines := splitLines(raw)
	c.log("RX %s", strings.Join(lines, " | "))
	return Response{Command: command, Lines: lines, Raw: raw, OK: ok}
}

func terminated(s string) (done, ok bool) {
	for _, line := range splitLines(s) {
		switch line {
		case "OK":
			return true, true
		case "ERROR":
			return true, false
		}
		if strings.HasPrefix(line, "+CME ERROR") || strings.HasPrefix(line, "+CMS ERROR") {
			return true, false
		}
	}
	return false, false
}

func splitLines(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == '\r' || r == '\n' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
