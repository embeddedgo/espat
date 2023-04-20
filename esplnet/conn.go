package esplnet

import (
	"io"
	"time"

	"github.com/embeddedgo/espat"
)

// A Conn represents a TCP or UDP conection.
type Conn struct {
	dev           *espat.Device
	conn          *espat.Conn
	readTimer     *time.Timer
	writeDeadline time.Time
	adata         []byte
	local         Addr
	remote        Addr
}

// DialDev works like the net.Dial function.
func DialDev(d *espat.Device, network, address string) (*Conn, error) {
	proto, host, _, port, err := splitHostPort(network, address)
	if err != nil {
		return nil, err
	}
	conn, err := d.CmdConn("+CIPSTARTEX=", proto, host, port)
	if err != nil {
		return nil, err
	}
	return newConn(d, conn)
}

func newConn(d *espat.Device, conn *espat.Conn) (*Conn, error) {
	sas, err := getSockAddrs(d)
	if err != nil {
		return nil, err
	}
	c := &Conn{dev: d, conn: conn, readTimer: time.NewTimer(0)}
	ci := conn.ID
	if ci < 0 {
		ci = 0
	}
	ci += '0' // maxConns must be <= 9
	for _, sa := range sas {
		if int(sa[0]) == ci {
			net, local, remote, _ := parseSockAddr(sa[2:])
			c.local.net = net
			c.local.hostPort = local
			c.remote.net = net
			c.remote.hostPort = remote
			break
		}
	}
	<-c.readTimer.C // unfortunately this is the only way to get a stopped timer
	return c, nil
}

// Read implements io.Reader interface.
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
	case <-c.readTimer.C: // timeout
		return 0, &espat.Error{c.dev.Name(), "read", espat.ErrTimeout}
	}
	// passive mode
	var args [3]any
	args[0] = p
	ai := 1
	if c.conn.ID >= 0 {
		args[ai] = c.conn.ID
		ai++
	}
	args[ai] = len(p)
	n, err = c.dev.CmdInt("+CIPRECVDATA=", args[:ai+1]...)
	return
}

func send(c *Conn, n int) (m int, err error) {
	var args [4]any
	ai := 0
	if c.conn.ID >= 0 {
		args[ai] = c.conn.ID
		ai++
	}
	c.dev.Lock()
	if !c.writeDeadline.IsZero() {
		to := int(c.writeDeadline.Sub(time.Now()) / time.Millisecond)
		if to <= 0 {
			err = &espat.Error{c.dev.Name(), "read", espat.ErrTimeout}
			return
		}
		args[ai+0] = -1
		args[ai+1] = 0
		args[ai+2] = to
		_, err = c.dev.UnsafeCmd("+CIPTCPOPT=", args[:ai+3]...)
		if err != nil {
			return
		}
	}
	m = n
	if m > 2048 {
		m = 2048
	}
	args[ai] = m
	_, err = c.dev.UnsafeCmd("+CIPSEND=", args[:ai+1]...)
	return
}

// Write implements io.Writer interface.
func (c *Conn) Write(p []byte) (n int, err error) {
	for len(p) != 0 {
		var m int
		m, err = send(c, len(p))
		if err == nil {
			m, err = c.dev.UnsafeWrite(p[:m])
			if err == nil {
				_, err = c.dev.UnsafeCmd("")
				n += m
				p = p[m:]
			}
		}
		c.dev.Unlock()
		if err != nil {
			break
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
			m, err = c.dev.UnsafeWriteString(p[:m])
			if err == nil {
				_, err = c.dev.UnsafeCmd("")
				n += m
				p = p[m:]
			}
		}
		c.dev.Unlock()
		if err != nil {
			break
		}
	}
	return
}

// Close works like the net.Conn Close method.
func (c *Conn) Close() error {
	var (
		args [1]any
		an   int
	)
	cmd := "+CIPCLOSE="
	if c.conn.ID < 0 {
		cmd = cmd[:len(cmd)-1]
	} else {
		args[0] = c.conn.ID
		an = 1
	}
	_, err := c.dev.Cmd(cmd, args[:an]...)
	return err
}

// SetReadDeadline works like the net.Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	tim := c.readTimer
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

// SetWriteDeadline works like the net.Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.dev.Lock() // use device mutex to avoid locking two mutexes in send
	c.writeDeadline = t
	c.dev.Unlock() // immediately unlocked so it shouldn't be very inefficient
	return nil
}

// LocalAddr works like the net.Conn LocalAddr method.
func (c *Conn) LocalAddr() *Addr {
	return &c.local
}

// RemoteAddr works like the net.Conn RemoteAddr method.
func (c *Conn) RemoteAddr() *Addr {
	return &c.remote
}
