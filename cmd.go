package espat

import (
	"errors"
	"io"
	"sync"
)

type cmd struct {
	name string
	args []any

	ready sync.Mutex
	resp  any
}

func processCmd(d *Device) {
	var buf [128]byte
	for cmd := range d.cmdq {
		if cmd.name != "" {
			err := writeCmd(d.w, &buf, cmd.name, cmd.args)
			if err != nil {
				cmd.resp = err
				cmd.ready.Unlock()
				continue
			}
		}
		d.receiver.cmd <- cmd
	}
}

func writeCmd(w io.Writer, buf *[128]byte, name string, args []any) error {
	buf[0] = 'A'
	buf[1] = 'T'
	n := 2
	n += copy(buf[n:], name)
	insert := func(c byte) {
		if n < len(buf) {
			buf[n] = c
			n++
		}
	}
	comma := false
	for i, arg := range args {
		if i == 0 {
			if _, ok := arg.([]byte); ok {
				continue // first argument can be a receive buffer
			}
		}
		if comma {
			insert(',')
		} else {
			comma = true
		}
		switch a := arg.(type) {
		case string:
			insert('"')
			for k := 0; k < len(a); k++ {
				c := a[k]
				if c == '"' || c == '\\' {
					insert('\\')
				}
				insert(c)
			}
			insert('"')
		case int:
			if a < 0 {
				insert('-')
				a = -a
			}
			switch {
			case a < 10:
				insert(byte(a + '0')) // fast path
			default:
				f := n
				for a != 0 {
					r := a % 10
					a /= 10
					insert(byte(r + '0'))
				}
				l := n - 1
				for f < l {
					buf[f], buf[l] = buf[l], buf[f]
					f++
					l--
				}
			}
		default:
			if arg != nil {
				return ErrArgType
			}
		}
	}
	if n > len(buf)-2 {
		return errors.New("Tx buffer overflow")
	}
	buf[n] = '\r'
	buf[n+1] = '\n'
	n += 2
	_, err := w.Write(buf[:n])
	return err
}
