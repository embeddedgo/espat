package espnet

import (
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/embeddedgo/espat"
)

// Conn is an implementation of the net.Conn interface for TCP and UDP network
// connections.
type Conn struct {
	d     *espat.Device
	conn  *espat.Conn
	net   string
	laddr netAddr
	raddr netAddr
	rtim  *time.Timer
	wdl   time.Time
	adata []byte
}

// DialDev works like net.Dial.
func DialDev(d *espat.Device, network, address string) (*Conn, error) {
	var proto6 string
	switch network {
	case "tcp", "tcp6", "tcp4":
		proto6 = "TCPv6"
	case "udp", "udp6", "udp4":
		proto6 = "UDPv6"
	default:
		return nil, net.UnknownNetworkError(network)
	}
	if len(address) == 0 {
		return nil, &net.AddrError{Err: "empty address"}
	}
	var (
		host, port string
		proto      = proto6
	)
	nlc := network[len(network)-1]
	if nlc != '6' {
		proto = proto6[:3]
	}
	if nlc != '4' && address[0] == '[' {
		i := strings.LastIndexByte(address, ']')
		if i >= 0 && i+1 < len(address) && address[i+1] == ':' {
			host, port = address[1:i], address[i+2:]
			proto = proto6
		}
	}
	if host == "" {
		i := strings.LastIndexByte(address, ':')
		if i < 0 {
			return nil, &net.AddrError{Err: "missing port in address", Addr: address}
		}
		host, port = address[:i], address[i+1:]
	}
	pn, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, &net.AddrError{Err: "unknown port", Addr: port}
	}
	conn, err := d.CmdConn("+CIPSTARTEX=", proto, host, int(pn))
	if err != nil {
		return nil, err
	}
	return newConn(d, conn, network, "")
}

func newConn(d *espat.Device, conn *espat.Conn, net, sport string) (*Conn, error) {
	sas, err := getSockAddrs(d)
	if err != nil {
		return nil, err
	}
	c := &Conn{d: d, conn: conn, net: net, rtim: time.NewTimer(0)}
	ci := conn.ID
	if ci < 0 {
		ci = 0
	}
	ci += '0' // maxConns must be <= 9
	for _, sa := range sas {
		if int(sa[0]) == ci {
			c.raddr, c.laddr.str, _ = parseSockAddr(sa[2:], sport)
			c.laddr.net = c.raddr.net
			break
		}
	}
	<-c.rtim.C // unfortunately this is the only way to get a stopped timer
	return c, nil
}

// Read implements the net.Conn Read method.
// BUG: Read cannot be used concurently in active mode.
func (c *Conn) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	if len(c.adata) != 0 {
		n = copy(p, c.adata)
		if n == len(c.adata) {
			c.adata = nil // ensure GC, help debugging
		} else {
			c.adata = c.adata[n:]
		}
		return
	}
	select {
	case data, ok := <-c.conn.Ch:
		if !ok {
			return n, io.EOF
		}
		if data != nil {
			// active mode
			n = copy(p, data)
			if n != len(data) {
				c.adata = data[n:]
			}
			return
		}
	case <-c.rtim.C: // timeout
		return 0, netOpError(c, "read", espat.ErrTimeout)
	}
	// passive mode
	var aa [3]any
	aa[0] = p
	ai := 1
	if c.conn.ID >= 0 {
		aa[ai] = c.conn.ID
		ai++
	}
	aa[ai] = len(p)
	n, err = c.d.CmdInt("+CIPRECVDATA=", aa[:ai+1]...)
	if err != nil {
		err = netOpError(c, "read", err)
	}
	return
}

func send(c *Conn, n int) (m int, err error) {
	var aa [4]any
	ai := 0
	if c.conn.ID >= 0 {
		aa[ai] = c.conn.ID
		ai++
	}
	c.d.Lock()
	if !c.wdl.IsZero() {
		to := int(c.wdl.Sub(time.Now()) / time.Millisecond)
		if to <= 0 {
			err = espat.ErrTimeout
			return
		}
		aa[ai+0] = -1
		aa[ai+1] = 0
		aa[ai+2] = to
		_, err = c.d.UnsafeCmd("+CIPTCPOPT=", aa[:ai+3]...)
		if err != nil {
			return
		}
	}
	m = n
	if m > 2048 {
		m = 2048
	}
	aa[ai] = m
	_, err = c.d.UnsafeCmd("+CIPSEND=", aa[:ai+1]...)
	return
}

// Write implements the net.Conn Write method.
func (c *Conn) Write(p []byte) (n int, err error) {
	for len(p) != 0 {
		var m int
		m, err = send(c, len(p))
		if err == nil {
			m, err = c.d.UnsafeWrite(p[:m])
			if err == nil {
				_, err = c.d.UnsafeCmd("")
				n += m
				p = p[m:]
			}
		}
		c.d.Unlock()
		if err != nil {
			err = netOpError(c, "write", err)
			return
		}
	}
	return
}

// WriteString implements io.StringWriter interface.
func (c *Conn) WriteString(p string) (n int, err error) {
	for len(p) != 0 {
		var m int
		m, err = send(c, len(p))
		if err == nil {
			m, err = c.d.UnsafeWriteString(p[:m])
			if err == nil {
				_, err = c.d.UnsafeCmd("")
				n += m
				p = p[m:]
			}
		}
		c.d.Unlock()
		if err != nil {
			err = netOpError(c, "write", err)
			return
		}
	}
	return
}

// Close implements the net.Conn Close method.
func (c *Conn) Close() error {
	var (
		err error
		aa  [1]any
		an  int
	)
	cmd := "+CIPCLOSE="
	if c.conn.ID < 0 {
		cmd = cmd[:len(cmd)-1]
	} else {
		aa[0] = c.conn.ID
		an = 1
	}
	_, err = c.d.Cmd(cmd, aa[:an]...)
	if err != nil {
		err = netOpError(c, "close", err)
	}
	return err
}

// SetReadDeadline implements the net.Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	tim := c.rtim
	if !tim.Stop() {
		select {
		case <-tim.C:
		default:
		}
	}
	if !t.IsZero() {
		tim.Reset(t.Sub(time.Now()))
	}
	return nil
}

// SetWriteDeadline implements the net.Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.d.Lock() // use device mutex to avoid locking two mutexes in send
	c.wdl = t
	c.d.Unlock() // immediately unlocked so it shouldn't be very inefficient
	return nil
}

// SetDeadline implements the net.Conn SetDeadline method.
func (c *Conn) SetDeadline(t time.Time) error {
	c.SetReadDeadline(t)
	c.SetWriteDeadline(t)
	return nil
}

// LocalAddr implements the net.Conn LocalAddr method.
func (c *Conn) LocalAddr() net.Addr {
	return &c.laddr
}

// RemoteAddr implements the net.Conn RemoteAddr method.
func (c *Conn) RemoteAddr() net.Addr {
	return &c.raddr
}
