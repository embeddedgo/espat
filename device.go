package espat

import (
	"io"
	"sync/atomic"
	"time"
)

// Device requires CIPDINFO=0.
type Device struct {
	name     string
	cmdq     chan *cmd
	w        io.Writer
	receiver receiver
}

// NewDevice returns a driver for ESP-AT device available via r and w. It also
// starts required background goroutines. You must call Init method before use
// the returned device.
func NewDevice(name string, r io.Reader, w io.Writer) *Device {
	d := &Device{name: name, cmdq: make(chan *cmd, 3), w: w}
	receiverInit(&d.receiver)
	go receiverLoop(&d.receiver, r)
	go processCmd(d)
	return d
}

// Ready reports whether the device is ready to accept AT commands.
func (d *Device) Ready() bool {
	return atomic.LoadInt32(&d.receiver.ready) != 0
}

// Async returns a channel that can be used to wait for asynchronous messages
// from ESP-AT device. They are called Active Message Reports in ESP-AT
// documentation and provide information of state changes like WiFi connect,
// disconnect, etc. If the async channel is full up two oldest messages are
// removed and an empty message is sent before a new one to inform about the
// channel overrun.
func (d *Device) Async() <-chan string {
	return d.receiver.async
}

func (d *Device) Server() <-chan *Conn {
	return d.receiver.server.Load().(chan *Conn)
}

func (d *Device) SetServer(en bool) {
	if en {
		d.receiver.server.Store(make(chan *Conn, maxConns))
	} else {
		d.receiver.server.Store((chan *Conn)(nil))
	}
}

// Init initailizes the device to the known state using the following commands:
//
//	ATE0
//	AT+SYSLOG=1
//
// If reset is true (recomended) it resets the device and waits for the ready
// state (for 3 second max.) before executing the above commands.
func (d *Device) Init(reset bool) error {
	if reset {
		// BUG: race with receiverLoop and restarting module, acceptable?
		if _, err := d.Cmd("+RST"); err != nil {
			return err
		}
		atomic.StoreInt32(&d.receiver.ready, 0)
		i, wait := 0, 3
		for ; i < wait; i++ {
			time.Sleep(time.Second)
			if d.Ready() {
				break
			}
		}
		if i == wait {
			return &Error{d.name, "ready", ErrTimeout}
		}
	}
	if _, err := d.Cmd("E0"); err != nil {
		return err
	}
	if _, err := d.Cmd("+SYSLOG=1"); err != nil {
		return err
	}
	atomic.StoreInt32(&d.receiver.ready, 1)
	return nil

}

// Cmd executes an AT command. Name should be a command name without the AT
// prefix (e.g. "+GMR" instead of "AT+GMR"). Args may be of type nil, string,
// int. The first argument can be also of type []byte and in a such case it may
// be used as a receive buffer (for example the CIPRECVDATA command may read
// data into it but is also allowed to discard all or part of received data if
// the buffer was missing or too small). If err is nil the returned response
// may be of type nil, string, int or *Conn. Use CmdStr, CmdInt, CmdConn if the
// response type is known in advance.
func (d *Device) Cmd(name string, args ...any) (resp any, err error) {
	c := &cmd{name: name, args: args}
	c.ready.Lock()
	d.cmdq <- c
	c.ready.Lock()
	if err, ok := c.resp.(error); ok {
		return nil, &Error{d.name, name, err}
	}
	return c.resp, nil
}

// CmdStr wraps Cmd. It checks the response type and returns an error if not a
// string.
func (d *Device) CmdStr(name string, args ...any) (string, error) {
	resp, err := d.Cmd(name, args...)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	if s, ok := resp.(string); ok {
		return s, nil
	}
	return "", &Error{d.name, name, ErrRespType}
}

// CmdInt wraps Cmd. It checks the response type and returns an error if not an
// int.
func (d *Device) CmdInt(name string, args ...any) (int, error) {
	resp, err := d.Cmd(name, args...)
	if err != nil {
		return 0, err
	}
	if i, ok := resp.(int); ok {
		return i, nil
	}
	return 0, &Error{d.name, name, ErrRespType}
}

// CmdConn wriaps Cmd. It checks the response type and returns an error if not a
// *Conn.
func (d *Device) CmdConn(name string, args ...any) (*Conn, error) {
	resp, err := d.Cmd(name, args...)
	if err != nil {
		return nil, err
	}
	if c, ok := resp.(*Conn); ok {
		return c, nil
	}
	return nil, &Error{d.name, name, ErrRespType}
}

func (d *Device) Write(p []byte) (int, error) {
	return d.w.Write(p)
}

func (d *Device) WriteString(s string) (int, error) {
	return io.WriteString(d.w, s)
}
