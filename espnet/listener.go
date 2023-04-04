package espnet

import (
	"net"
	"strconv"
)

type Listener struct {
	d *Device
	a netAddr
}

func ListenDev(d *Device, network string, port int) (*Listener, error) {
	dev := d.dev
	if _, err := dev.Cmd("+CIPMUX=1"); err != nil {
		return nil, err
	}
	dev.SetServer(true)
	if _, err := dev.Cmd("+CIPSERVER=1,", port); err != nil {
		return nil, err
	}
	return &Listener{d, netAddr{network, ":" + strconv.Itoa(port)}}, nil
}

func (ls *Listener) Accept() (net.Conn, error) {
	return newConn(ls.d, <-ls.d.dev.Server(), ls.a.net, ls.a.str)
}

func (ls *Listener) Close() error {
	return nil
}

func (ls *Listener) Addr() net.Addr {
	return &ls.a
}
