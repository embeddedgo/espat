package espnet

import (
	"net"
	"strconv"

	"github.com/embeddedgo/espat"
)

type Listener struct {
	d *espat.Device
	a netAddr
}

func ListenDev(d *espat.Device, network string, port int) (*Listener, error) {
	if _, err := d.Cmd("+CIPMUX=1"); err != nil {
		return nil, err
	}
	d.SetServer(true)
	if _, err := d.Cmd("+CIPSERVER=1,", port); err != nil {
		return nil, err
	}
	return &Listener{d, netAddr{network, ":" + strconv.Itoa(port)}}, nil
}

func (ls *Listener) Accept() (net.Conn, error) {
	c := <-ls.d.Server()
	return newConn(ls.d, c, ls.a.net, ls.a.str)
}

func (ls *Listener) Close() error {
	_, err := ls.d.Cmd("+CIPSERVER=0,1")
	return err
}

func (ls *Listener) Addr() net.Addr {
	return &ls.a
}
