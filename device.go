package espat

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// Device requires CIPDINFO=0.
type Device struct {
	name     string
	cmdq     chan *cmd
	cmdx     sync.Mutex
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

// Server returns the server channel. See also SetServer.
func (d *Device) Server() <-chan *Conn {
	return d.receiver.server.Load().(chan *Conn)
}

// SetServer enables the server channel. See also Server.
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

// Lock locks the device. Device should be locked before use UnsafeCmd, Write,
// WriteString methods.
func (d *Device) Lock() {
	d.cmdx.Lock()
}

// Unlock unlocks the device.
func (d *Device) Unlock() {
	d.cmdx.Unlock()
}

func exec(d *Device, safe bool, name string, args []any) (resp any, err error) {
	c := &cmd{name: name, args: args}
	c.ready.Lock()
	if safe {
		d.cmdx.Lock()
	}
	d.cmdq <- c
	if safe {
		d.cmdx.Unlock()
	}
	c.ready.Lock()
	if err, ok := c.resp.(error); ok {
		return nil, &Error{d.name, name, err}
	}
	return c.resp, nil
}

// Cmd executes an AT command. Name should be a command name without the AT
// prefix (e.g. "+GMR" instead of "AT+GMR"). Args may be of type nil, string,
// int. The first argument can be also of type []byte and in a such case it may
// be used as a receive buffer (for example the CIPRECVDATA command may read
// data into it but is also allowed to discard all or part of received data if
// the buffer was missing or too small). If err is nil the returned response
// may be of type nil, string, int or *Conn. Use CmdStr, CmdInt if the
// response type is known in advance.
func (d *Device) Cmd(name string, args ...any) (resp any, err error) {
	return exec(d, true, name, args)
}

// CmdStr provides a convenient way to execute a command when a string response
// is expected.
func (d *Device) CmdStr(name string, args ...any) (string, error) {
	resp, err := exec(d, true, name, args)
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

// CmdInt provides a convenient way to execute a command when an int response
// is expected.
func (d *Device) CmdInt(name string, args ...any) (int, error) {
	resp, err := exec(d, true, name, args)
	if err != nil {
		return 0, err
	}
	if i, ok := resp.(int); ok {
		return i, nil
	}
	return 0, &Error{d.name, name, ErrRespType}
}

// CmdConn provides a convenient way to execute a command when a *Conn response
// is expected.
func (d *Device) CmdConn(name string, args ...any) (*Conn, error) {
	resp, err := exec(d, true, name, args)
	if err != nil {
		return nil, err
	}
	if c, ok := resp.(*Conn); ok {
		return c, nil
	}
	return nil, &Error{d.name, name, ErrRespType}
}

// UnsafeCmd is like Cmd but intended to be used with a locked device.
func (d *Device) UnsafeCmd(name string, args ...any) (resp any, err error) {
	return exec(d, false, name, args)
}

// UnsafeWrite works like io.Writer Write method. Device must be locked and
// ready for at least len(p) bytes of data.
func (d *Device) UnsafeWrite(p []byte) (int, error) {
	return d.w.Write(p)
}

// UnsafeWriteString works like io.StringWriter WriteString method. Device must
// be locked and ready for at least len(s) bytes of data.
func (d *Device) UnsafeWriteString(s string) (int, error) {
	return io.WriteString(d.w, s)
}
