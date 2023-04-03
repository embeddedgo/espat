package espnet

import "net"

func netOpError(c *Conn, op string, err error) error {
	return &net.OpError{
		Op:     op,
		Net:    c.net,
		Source: &c.laddr,
		Addr:   &c.raddr,
		Err:    err,
	}
}
