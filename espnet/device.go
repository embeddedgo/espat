package espnet

import (
	"io"
	"sync"

	"github.com/embeddedgo/espat"
)

type Device struct {
	dev *espat.Device
	mt  sync.Mutex
}

func NewDevice(name string, r io.Reader, w io.Writer) *Device {
	return &Device{dev: espat.NewDevice(name, r, w)}
}

// Init options
const (
	Reboot     = 1 << 0 // reboot the device (recommended)
	SingleConn = 1 << 1 // single connection mode
	ActiveRecv = 1 << 2 // active receive mode (not recommended)
)

func (d *Device) Init(options int) error {
	if err := d.dev.Init(options&Reboot != 0); err != nil {
		return err
	}
	if _, err := d.dev.Cmd("+CIPMUX=", ^options>>1&1); err != nil {
		return err
	}
	if _, err := d.dev.Cmd("+CIPRECVMODE=", ^options>>2&1); err != nil {
		return err
	}
	return nil
}

func (d *Device) ESPAT() *espat.Device {
	return d.dev
}
