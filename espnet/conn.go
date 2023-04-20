package espnet

import (
	"net"
	"time"

	"github.com/embeddedgo/espat"
	"github.com/embeddedgo/espat/esplnet"
)

// Conn is an implementation of the net.Conn interface for TCP and UDP network
// connections.
type Conn esplnet.Conn

// DialDev works like net.Dial.
func DialDev(d *espat.Device, network, address string) (*Conn, error) {
	c, err := esplnet.DialDev(d, network, address)
	if err != nil {
		if _, ok := err.(*espat.Error); !ok {
			s := err.Error()
			if s == "unknown network " {
				err = net.UnknownNetworkError(network)
			} else {
				err = &net.AddrError{Err: s}
			}
		}
		return nil, &net.OpError{Op: "dial", Net: network, Err: err}
	}
	return (*Conn)(c), nil
}

// Read implements the net.Conn Read method.
// BUG: Read cannot be used concurently in active mode.
func (c *Conn) Read(p []byte) (n int, err error) {
	n, err = (*esplnet.Conn)(c).Read(p)
	return n, netOpError(c, "read", err)
}

// Write implements the net.Conn Write method.
func (c *Conn) Write(p []byte) (n int, err error) {
	n, err = (*esplnet.Conn)(c).Write(p)
	return n, netOpError(c, "write", err)
}

// WriteString implements io.StringWriter interface.
func (c *Conn) WriteString(p string) (n int, err error) {
	n, err = (*esplnet.Conn)(c).WriteString(p)
	return n, netOpError(c, "write", err)
}

// Close implements the net.Conn Close method.
func (c *Conn) Close() error {
	return netOpError(c, "close", (*esplnet.Conn)(c).Close())
}

// SetReadDeadline implements the net.Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return (*esplnet.Conn)(c).SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return (*esplnet.Conn)(c).SetWriteDeadline(t)
}

// SetDeadline implements the net.Conn SetDeadline method.
func (c *Conn) SetDeadline(t time.Time) error {
	(*Conn)(c).SetReadDeadline(t)
	return (*esplnet.Conn)(c).SetWriteDeadline(t)
}

// LocalAddr implements the net.Conn LocalAddr method.
func (c *Conn) LocalAddr() net.Addr {
	return (*esplnet.Conn)(c).LocalAddr()
}

// RemoteAddr implements the net.Conn RemoteAddr method.
func (c *Conn) RemoteAddr() net.Addr {
	return (*esplnet.Conn)(c).RemoteAddr()
}
