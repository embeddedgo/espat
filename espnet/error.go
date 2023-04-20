package espnet

import (
	"net"
)

func netOpError(c *Conn, op string, err error) error {
	if err != nil {
		local := c.LocalAddr()
		remote := c.RemoteAddr()
		err = &net.OpError{
			Op:     op,
			Net:    local.Network(),
			Source: local,
			Addr:   remote,
			Err:    err,
		}
	}
	return err
}
