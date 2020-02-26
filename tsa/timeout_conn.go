package tsa

import (
	"net"
	"time"
)

// This implements an idle timeout between operations (read or write )
// FROM net.go
//   |
//    -> An idle timeout can be implemented by repeatedly extending
//    -> the deadline after successful Read or Write calls.
type IdleTimeoutConn struct {
	net.Conn
	IdleTimeout time.Duration
}

func (c *IdleTimeoutConn) Write(p []byte) (int, error) {
	i, err := c.Conn.Write(p)

	if err == nil {
		c.updateDeadline()
	}
	return i, err
}

func (c *IdleTimeoutConn) Read(b []byte) (int, error) {
	i, err := c.Conn.Read(b)

	if err == nil {
		c.updateDeadline()
	}
	return i, err
}

func (c *IdleTimeoutConn) updateDeadline() {
	idleDeadline := time.Now().Add(c.IdleTimeout)
	c.Conn.SetDeadline(idleDeadline)
}
