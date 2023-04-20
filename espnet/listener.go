package espnet

import (
	"net"

	"github.com/embeddedgo/espat"
	"github.com/embeddedgo/espat/esplnet"
)

// Listener wraps esplnet.Listener to implement the net.Listener interface.
type Listener esplnet.Listener

// ListenDev works like the net.Listen function.
func ListenDev(d *espat.Device, network, address string) (*Listener, error) {
	ls, err := esplnet.ListenDev(d, network, address)
	if err != nil {
		return nil, &net.OpError{Op: "listen", Net: network, Err: err}
	}
	return (*Listener)(ls), nil
}

func (ls *Listener) Addr() net.Addr {
	return (*esplnet.Listener)(ls).Addr()
}

func (ls *Listener) Accept() (net.Conn, error) {
	c, err := (*esplnet.Listener)(ls).Accept()
	conn := (*Conn)(c)
	return conn, netOpError(conn, "accept", err)
}

func (ls *Listener) Close() error {
	err := (*esplnet.Listener)(ls).Close()
	if err != nil {
		err = &net.OpError{Op: "close", Net: ls.Addr().Network(), Err: err}
	}
	return err
}
