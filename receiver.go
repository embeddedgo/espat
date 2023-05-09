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
	Dev *Device
	ID  int           // connection ID or -1
	Ch  <-chan []byte // receive channel
}

// Async represents an asynchronous message from the ESP-AT device or
// asynchronous receive error. The messages are called Active Message Reports
// in ESP-AT documentation and provide information of state changes like WiFi
// connect, disconnect, etc.
type Async struct {
	Str string
	Err error
}

// maxConns is the number of open connections supported by the ESP-AT. While
// writing this code the ESP-AT supports up to 5 open connections but we support
// up to 10 and the parser assumes a connection id is a single digit number. If
// the ESP-AT increases the number of supported connections over 10, a change
// in receiverLoop will be required.
const maxConns = 10 // keep in sync with espnet/sockaddr.go

type receiver struct {
	cmd    chan *cmd
	async  chan Async
	server atomic.Value
	conns  [maxConns]chan []byte
}

func receiverInit(rcv *receiver) {
	rcv.cmd = make(chan *cmd)
	rcv.async = make(chan Async, 5)
}

func receiverLoop(dev *Device, inp io.Reader) {
	var (
		sb    strings.Builder
		emptl bool
		resp  Response
		rerr  error
	)
	rcv := &dev.receiver
	r := bufio.NewReaderSize(inp, 128)
	for {
		line, err := r.ReadSlice('\n')
		if err != nil && err != bufio.ErrBufferFull {
			rerr = err
			goto sendAsync
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
				rerr = ErrParse
				goto sendAsync
			}
			conn := rcv.conns[ci]
			if conn == nil {
				rerr = ErrUnkConn
				goto sendAsync
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
					rerr = ErrParse
					goto sendAsync
				}
				pkt := make([]byte, m)
				if err = readData(line[k+1:], r, pkt, m); err != nil {
					rerr = ErrParse
					goto sendAsync
				}
				conn <- pkt
				continue
			case 1: // passive
				conn <- nil
				continue
			default:
				rerr = ErrParse
				goto sendAsync
			}
		case len(line) > 15 && string(line[:13]) == "+CIPRECVDATA:":
			k := 14
			for ; k < len(line); k++ {
				if line[k] == ',' {
					break
				}
			}
			if k == len(line) {
				rerr = ErrParse
				goto sendAsync
			}
			m, _ := strconv.Atoi(string(line[13:k]))
			if m <= 0 {
				rerr = ErrParse
				goto sendAsync
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
				cmd.err = err
			} else if _, err = r.ReadSlice('\n'); err != nil {
				cmd.err = err
			} else {
				cmd.resp.Int = len(buf)
			}
			cmd.ready.Unlock()
			continue
		}
		if n := len(line); err == bufio.ErrBufferFull || n < 2 || line[n-2] != '\r' {
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
					if s != "" {
						resp.Str = s
					}
				} else {
					if s == "" {
						s = "socket"
					}
					rerr = &ErrorESP{s}
				}
				goto sendResp
			}
			if ok := string(line) == "SEND OK"; ok || string(line) == "SEND FAIL" {
				if !ok {
					rerr = ErrTimeout // BUG? is "SEND FAIL" always a timeout?
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
					rerr = ErrParse
					goto sendAsync
				}
				id = ci
			}
			if line[len(line)-1] == 'T' {
				// CONNECT
				ch := make(chan []byte, 3)
				rcv.conns[ci] = ch
				conn := &Conn{dev, id, ch}
				if srv := rcv.server.Load(); srv != nil {
					srv.(chan *Conn) <- conn
				} else {
					resp.Conn = conn
				}
			} else {
				// CLOSED
				ch := rcv.conns[ci]
				if ch != nil {
					close(ch)
					rcv.conns[ci] = nil
				}
			}
		case string(line) == "ready":
			goto sendAsync
		case len(line) > 5 && string(line[:5]) == "WIFI ":
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
			cmd.err = rerr
			cmd.ready.Unlock()
			resp = Response{}
			rerr = nil
			sb.Reset()
			continue
		}
	sendAsync:
		{
			msg := Async{string(line), rerr}
			rerr = nil
			overrun := false
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
				rcv.async <- Async{} // inform about an overrun
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
