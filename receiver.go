package espat

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
)

// Conn represents a TCP or UDP connection.
//
// The ID field is the connection ID in multiple connection mode or -1 in single
// connection mode.
//
// The Ch field is the channel that returns received data in active receive mode
// or informs about the availability of new data (returning nil) in passive
// receive mode.
type Conn struct {
	ID int           // connection ID or -1
	Ch <-chan []byte // receive channel
}

// maxConns is the number of open connections supported by the ESP-AT. While
// writing this code the ESP-AT supports up to 5 open connections but we support
// up to 10 and the parser assumes a connection id is a single digit number. If
// the ESP-AT increases the number of supported connections over 10, a change
// in receiverLoop will be required.
const maxConns = 10 // keep in sync with espnet/sockaddr.go

type receiver struct {
	cmd    chan *cmd
	async  chan string
	server atomic.Value
	conns  [maxConns]chan []byte
	ready  int32
}

func receiverInit(rcv *receiver) {
	rcv.cmd = make(chan *cmd)
	rcv.async = make(chan string, 5)
}

func receiverLoop(rcv *receiver, inp io.Reader) {
	var (
		sb    strings.Builder
		emptl bool
		resp  any
	)
	r := bufio.NewReaderSize(inp, 128)
	for {
		line, err := r.ReadSlice('\n')
		if err != nil && err != bufio.ErrBufferFull {
			resp = err
			goto sendResp
		}
		switch {
		case len(line) >= 7 && string(line[:5]) == "+IPD,":
			ci := 0
			i := 5
			if line[6] == ',' {
				// CIPMUX=1
				ci = int(line[5]) - '0'
				i = 7
			}
			if uint(ci) >= maxConns {
				resp = ErrParse
				goto sendResp
			}
			conn := rcv.conns[ci]
			if conn == nil {
				resp = ErrUnkConn
				goto sendResp
			}
			recvmode := -1
			k := i + 1
			// find ':' or "\r\n"
			for ; k < len(line); k++ {
				if c := line[k]; c == ':' {
					recvmode = 0
					break
				} else if c == '\r' && k+1 < len(line) && line[k+1] == '\n' {
					recvmode = 1
					break
				}
			}
			switch recvmode {
			case 0: // active
				m, _ := strconv.Atoi(string(line[i:k]))
				if m <= 0 {
					close(conn) // avoid deadlock
					resp = ErrParse
					goto sendResp
				}
				pkt := make([]byte, m)
				if err = readData(line[k+1:], r, pkt, m); err != nil {
					resp = ErrParse
					goto sendResp
				}
				conn <- pkt
				continue
			case 1: // passive
				conn <- nil
				continue
			default:
				resp = ErrParse
				goto sendResp
			}
		case len(line) > 15 && string(line[:13]) == "+CIPRECVDATA:":
			k := 14
			for ; k < len(line); k++ {
				if line[k] == ',' {
					break
				}
			}
			if k == len(line) {
				resp = ErrParse
				goto sendResp
			}
			m, _ := strconv.Atoi(string(line[13:k]))
			if m <= 0 {
				resp = ErrParse
				goto sendResp
			}
			cmd := <-rcv.cmd
			var buf []byte
			if len(cmd.args) != 0 {
				buf, _ = cmd.args[0].([]byte)
			}
			if len(buf) > m {
				buf = buf[:m] // readData requires len(buf) <= m
			}
			if err = readData(line[k+1:], r, buf, m); err != nil {
				cmd.resp = err
			} else if _, err = r.ReadSlice('\n'); err != nil {
				cmd.resp = err
			} else {
				cmd.resp = len(buf)
			}
			cmd.ready.Unlock()
			resp = nil
			sb.Reset()
			continue
		}
		if err == bufio.ErrBufferFull || line[len(line)-2] != '\r' {
			sb.Write(line)
			continue
		}
		line = line[:len(line)-2]
		if len(line) == 0 {
			emptl = true
			continue
		}
		if emptl {
			emptl = false
			if ok := string(line) == "OK"; ok || string(line) == "ERROR" {
				s := sb.String()
				if ok {
					if resp == nil && s != "" {
						resp = s
					}
				} else {
					if s == "" {
						s = "socket"
					}
					resp = &ErrorESP{s}
				}
				goto sendResp
			}
			if ok := string(line) == "SEND OK"; ok || string(line) == "SEND FAIL" {
				if !ok {
					resp = ErrTimeout // BUG? is "SEND FAIL" always a timeout?
				}
				goto sendResp
			}
		}
		switch {
		case string(line) == ">":
			// skip a prompt sign
		case len(line) >= 12 && string(line[:5]) == "Recv ":
			// skip ESP-AT confirmation of data receipt
		case string(line) == "CONNECT" || string(line) == "CLOSED" ||
			string(line[1:]) == ",CONNECT" || string(line[1:]) == ",CLOSED":
			id := -1
			ci := 0
			if c := line[0]; c != 'C' {
				// CIPMUX=1
				ci = int(c) - '0'
				if uint(ci) >= maxConns {
					resp = ErrParse
					goto sendResp
				}
				id = ci
			}
			if line[len(line)-1] == 'T' {
				// CONNECT
				ch := make(chan []byte, 3)
				rcv.conns[ci] = ch
				conn := &Conn{id, ch}
				if srv := rcv.server.Load(); srv != nil {
					srv.(chan *Conn) <- conn
				} else {
					resp = conn
				}
			} else {
				// CLOSED
				ch := rcv.conns[ci]
				if ch != nil {
					close(ch)
					rcv.conns[ci] = nil
				}
			}
		case len(line) > 5 && string(line[:5]) == "WIFI ":
			goto sendAsync
		case string(line) == "ready":
			atomic.StoreInt32(&rcv.ready, 1)
			goto sendAsync
		default:
			sb.Grow(len(line) + 1)
			sb.Write(line)
			sb.WriteByte('\n')
		}
		continue
	sendResp:
		{
			cmd := <-rcv.cmd
			cmd.resp = resp
			cmd.ready.Unlock()
			resp = nil
			sb.Reset()
		}
		continue
	sendAsync:
		{
			if len(line) == 0 {
				continue
			}
			overrun := false
			msg := string(line)
		again:
			select {
			case rcv.async <- msg:
				continue
			default:
			}
			// Async channel is full. Remove the oldest message.
			select {
			case <-rcv.async:
			default:
			}
			if !overrun {
				overrun = true
				rcv.async <- "" // inform about an overrun
			}
			goto again
		}
	}
}

// readData reads m bytes from the preread and r. The first len(buf) read bytes
// are placed into buf. len(buf) must be <= m.
func readData(preread []byte, r *bufio.Reader, buf []byte, m int) error {
	n := copy(buf, preread)
	preread = preread[n:]
	for n < len(buf) {
		k, err := r.Read(buf[n:])
		if err != nil {
			return err
		}
		n += k
	}
	m -= n
	if m >= 0 {
		n = len(preread)
		if n > m {
			n = m
		}
		preread = preread[n:]
		m -= n
	}
	if m != 0 {
		if _, err := r.Discard(m); err != nil {
			return err
		}
	}
	if string(preread) != "\r\n" {
		line, err := r.ReadSlice('\n')
		if err != nil {
			return err
		}
		if line[len(line)-2] != '\r' {
			return ErrParse
		}
	}
	return nil
}
