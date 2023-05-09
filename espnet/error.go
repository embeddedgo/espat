package espnet

import (
	"io"
	"net"
)

func netOpError(c *Conn, op string, err error) error {
	if err != nil && err != io.EOF {
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
