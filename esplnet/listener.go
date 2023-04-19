package esplnet

import (
	"errors"
	"strconv"
	"strings"

	"github.com/embeddedgo/espat"
)

type Listener struct {
	d *espat.Device
	a Addr
}

// ListenDev works like the net.Listen function.
func ListenDev(d *espat.Device, network, address string) (*Listener, error) {
	i := strings.LastIndexByte(address, ':')
	if i < 0 {
		return nil, errors.New("missing port in address")
	}
	u, _ := strconv.ParseUint(address[i+1:], 10, 16)
	port := int(u)
	if port == 0 {
		return nil, errors.New("unknown port")
	}
	if _, err := d.Cmd("+CIPMUX=1"); err != nil {
		return nil, err
	}
	d.SetServer(true)
	if _, err := d.Cmd("+CIPSERVER=1,", port); err != nil {
		return nil, err
	}
	ls := new(Listener)
	ls.d = d
	ls.a.net = network
	ls.a.hostPort = address
	return ls, nil
}

// Accept works like the net.Listener Accept method.
func (ls *Listener) Accept() (*Conn, error) {
	c := <-ls.d.Server()
	return newConn(ls.d, c)
}

// Close works like the net.Listener Close method.
func (ls *Listener) Close() error {
	_, err := ls.d.Cmd("+CIPSERVER=0,1")
	return err
}

// Addr works like the net.Listener Addr method.
func (ls *Listener) Addr() *Addr {
	return &ls.a
}
