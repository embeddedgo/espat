// Simpleserver is a simple TCP-only server.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/embeddedgo/espat"
	"github.com/embeddedgo/espat/espn"
	"github.com/ziutek/serial"
)

func logErr(err error) bool {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return true
	}
	return false
}

func fatalErr(err error) {
	if logErr(err) {
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage:")
		fmt.Println("  simpleserver UART_DEVICE HOST:PORT")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  simpleserver /dev/ttyUSB0 :1234")
		os.Exit(1)
	}

	// Setup the UART interface.
	uart, err := serial.Open(os.Args[1])
	fatalErr(err)
	fatalErr(uart.SetSpeed(115200))

	// Initialize the ESP-AT device.
	dev := espat.NewDevice("esp0", uart, uart)
	fatalErr(dev.Init(true))

waitForIP:
	for {
		select {
		case msg := <-dev.Async():
			fatalErr(msg.Err)
			if msg.Str == "WIFI GOT IP" {
				break waitForIP
			}
		case <-time.After(5 * time.Second):
			fmt.Println("Cannot obtain an IP address: timeout.")
			os.Exit(1)
		}
	}

	ls, err := espn.ListenDev(dev, "tcp", os.Args[2])
	fatalErr(err)

	fmt.Println("Waiting for TCP connections...")
	for {
		c, err := ls.Accept()
		fatalErr(err)
		go handle(c)
	}
}

func handle(c *espn.Conn) {
	fmt.Printf("- new connection: %s -> %s\n", c.RemoteAddr(), c.LocalAddr())
	for {
		_, err := fmt.Fprint(c, "Enter two numbers separated by a space: ")
		if logErr(err) {
			return
		}
		var a, b float64
		_, err = fmt.Fscanf(c, "%g %g\n", &a, &b)
		if err != nil {
			if err == io.EOF {
				// There is no need to call Close after reading io.EOF because
				// there are no any connection-related resources in espn (like
				// file descriptors) that might not be released by GC. However,
				// it is recommended for compatibility with the Go net package.
				logErr(c.Close())
			} else {
				logErr(err)
			}
			return
		}
		_, err = fmt.Fprintf(c, "\na=%g b=%g a+b=%g a*b=%g\n\n", a, b, a+b, a*b)
		logErr(err)
	}
}
