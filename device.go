package espat

import (
	"io"
	"sync"
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
	d.cmdx.Lock() // to delay Init(true), will be unlocked by receiverLoop
	receiverInit(&d.receiver)
	go receiverLoop(d, r)
	go processCmd(d)
	return d
}

// Name returns the device name set by NewDevice.
func (d *Device) Name() string {
	return d.name
}

// Async returns a channel that can be used to wait for asynchronous messages
// from ESP-AT device. The channel overflow is signaled by sending an empty
// message. In such case up two oldest messages are removed from the channel and
// the received message is placed right after the empty one.
func (d *Device) Async() <-chan Async {
	return d.receiver.async
}

// Server returns the server channel. See also SetServer.
func (d *Device) Server() <-chan *Conn {
	return *d.receiver.server.Load()
}

// SetServer enables the server channel. See also Server.
func (d *Device) SetServer(en bool) {
	if en {
		c := make(chan *Conn, maxConns)
		d.receiver.server.Store(&c)
	} else {
		d.receiver.server.Store(nil)
	}
}

// Init initailizes the device to the known state using the following commands:
//
//	ATE0
//	AT+SYSLOG=1
//
// If reset is true (recomended) it resets the device and waits for the ready
// state (2 second max.) before executing the above commands.
func (d *Device) Init(reset bool) error {
	if reset {
		d.cmdx.Lock()
		timeout := time.After(50 * time.Millisecond)
	emptying:
		for {
			select {
			case <-d.Async():
				//
			case <-timeout:
				break emptying
			}
		}
		d.cmdx.Unlock()
		if _, err := d.Cmd("+RST"); err != nil {
			return err
		}
		timeout = time.After(2 * time.Second)
	waiting:
		for {
			select {
			case msg := <-d.Async():
				// There may be many noise-like errors afer reset, ignore all.
				if msg.Str == "ready" {
					break waiting
				}
			case <-timeout:
				return &Error{d.name, "ready", ErrTimeout}
			}
		}
	}
	if _, err := d.Cmd("E0"); err != nil {
		return err
	}
	if _, err := d.Cmd("+SYSLOG=1"); err != nil {
		return err
	}
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

// Cmd executes an AT command. Name should be a command name without the AT
// prefix (e.g. "+GMR" instead of "AT+GMR"). Args may be of type string, int or
// nil. The first argument can be also of type []byte and in a such case it may
// be used as a receive buffer (for example the CIPRECVDATA command may read
// data into it but is also allowed to discard all or part of received data if
// the buffer was missing or too small). CmdStr, CmdInt, CmdConn can be used
// instead of Cmd if the response type is known in advance.
func (d *Device) Cmd(name string, args ...any) (resp *Response, err error) {
	c := &cmd{name: name, args: args}
	c.ready.Lock()
	d.cmdx.Lock()
	d.cmdq <- c
	d.cmdx.Unlock()
	c.ready.Lock()
	if c.err != nil {
		err = &Error{d.name, name, c.err}
	}
	return &c.resp, err
}

// UnsafeCmd is like Cmd but intended to be used with a locked device.
func (d *Device) UnsafeCmd(name string, args ...any) (resp *Response, err error) {
	c := &cmd{name: name, args: args}
	c.ready.Lock()
	d.cmdq <- c
	c.ready.Lock()
	if c.err != nil {
		err = &Error{d.name, name, c.err}
	}
	return &c.resp, err
}

// CmdStr provides a convenient way to execute a command when a string response
// is expected.
func (d *Device) CmdStr(name string, args ...any) (string, error) {
	resp, err := d.Cmd(name, args...)
	return resp.Str, err
}

// CmdInt provides a convenient way to execute a command when an int response
// is expected.
func (d *Device) CmdInt(name string, args ...any) (int, error) {
	resp, err := d.Cmd(name, args...)
	return resp.Int, err
}

// CmdConn provides a convenient way to execute a command when a *Conn response
// is expected.
func (d *Device) CmdConn(name string, args ...any) (*Conn, error) {
	resp, err := d.Cmd(name, args...)
	return resp.Conn, err
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
