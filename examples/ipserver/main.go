// Ipserver a is an example TCP server written using raw espat package.
// In most cases you probably don't want to write a server using AT commands
// directly like in this example. Use espnet package instead and see
// ../../espnet/examples for more real-world examples.
// Ipserver expects configured Wi-Fi on the ESP-AT device (see ../wifisetup).
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/embeddedgo/espat"
	"github.com/ziutek/serial"
)

func logErr(err error) bool {
	if err == nil {
		return false
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	return true
}

func fatalErr(err error) {
	if logErr(err) {
		os.Exit(1)
	}
}

func main() {
	var (
		fa = flag.Bool("a", false, "active receive mode (CIPRECVMODE=0)")
		fb = flag.Int("b", 115200, "baudrate")
		fr = flag.Bool("r", false, "reboot the ESP-AT device first")
	)
	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  ipclient [options] UART_DEVICE TCP_PORT")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ipserver -r /dev/ttyUSB0 1234")
	}

	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}
	port, err := strconv.ParseUint(flag.Arg(1), 10, 16)
	if err != nil {
		fatalErr(fmt.Errorf("bad TCP port: %w", err))
	}
	passive := 1
	if *fa {
		passive = 0
	}

	uart, err := serial.Open(flag.Arg(0))
	fatalErr(err)
	fatalErr(uart.SetSpeed(*fb))
	d := espat.NewDevice("esp0", uart, uart)
	fatalErr(d.Init(*fr))

	if *fr {
		for msg := range d.Async() {
			if msg == "WIFI GOT IP" {
				break
			}
		}
	}

	_, err = d.Cmd("+CIPMUX=1")
	fatalErr(err)
	_, err = d.Cmd("+CIPRECVMODE=", passive)
	fatalErr(err)
	d.SetServer(true)
	_, err = d.Cmd("+CIPSERVER=1,", int(port))
	fatalErr(err)

	fmt.Println("Waiting for TCP connections...")

	for conn := range d.Server() {
		go handle(d, conn, *fa)
	}
}

var welcome = []byte("Welcome to the Echo Server!\r\n")

func handle(d *espat.Device, conn *espat.Conn, active bool) {
	err := send(d, conn, welcome)
	if logErr(err) {
		return
	}
	if active {
		for {
			data, ok := <-conn.Ch
			if !ok {
				return // connection closed by remote part
			}
			if logErr(send(d, conn, data)) {
				return
			}
		}
	} else {
		var buf [128]byte
		for {
			if _, ok := <-conn.Ch; !ok {
				break // connection closed by remote part
			}
			n, err := d.CmdInt("+CIPRECVDATA=", buf[:], conn.ID, len(buf))
			if logErr(err) {
				return
			}
			if logErr(send(d, conn, buf[:n])) {
				return
			}
		}
	}
}

func send(d *espat.Device, conn *espat.Conn, p []byte) error {
	d.Lock()
	defer d.Unlock()
	if _, err := d.UnsafeCmd("+CIPSEND=", conn.ID, len(p)); err != nil {
		return err
	}
	if _, err := d.UnsafeWrite(p); err != nil {
		return err
	}
	_, err := d.UnsafeCmd("")
	return err
}
