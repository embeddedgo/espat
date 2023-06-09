// Copyright 2023 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package espn

import "github.com/embeddedgo/espat"

// SetMultiConn enables/disables the multiple connection mode.
func SetMultiConn(d *espat.Device, multiConn bool) error {
	a := 0
	if multiConn {
		a = 1
	}
	_, err := d.Cmd("+CIPMUX=", a)
	return err
}

// SetPasvRecv enables/disables the passive receive mode.
func SetPasvRecv(d *espat.Device, pasvRecv bool) error {
	a := 0
	if pasvRecv {
		a = 1
	}
	_, err := d.Cmd("+CIPRECVMODE=", a)
	return err
}
