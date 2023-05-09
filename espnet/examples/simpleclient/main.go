// Simpleclient is a simple TCP-only client. See ../client for more feature rich
// TCP/UDP one.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/embeddedgo/espat"
	"github.com/embeddedgo/espat/espnet"
	"github.com/ziutek/serial"
)

func fatalErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage:")
		fmt.Println("  simpleclient UART_DEVICE IP_ADDR:PORT")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  simpleclient /dev/ttyUSB0 192.168.1.100:1234")
		fmt.Println("  simpleclient /dev/ttyUSB0 test.server.local:1234")
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

	conn, err := espnet.DialDev(dev, "tcp", os.Args[2])
	fatalErr(err)

	// Sender
	go func() {
		var buf [1024]byte
		for {
			n, err := os.Stdin.Read(buf[:])
			if n != 0 {
				_, err = conn.Write(buf[:n])
				fatalErr(err)
			}
			if err == io.EOF {
				fatalErr(conn.Close())
				return
			}
			fatalErr(err)
		}
	}()

	// Receiver
	var buf [1024]byte
	for {
		n, err := conn.Read(buf[:])
		if n != 0 {
			_, err := os.Stdout.Write(buf[:n])
			fatalErr(err)
		}
		if err != nil {
			if err == io.EOF {
				fatalErr(conn.Close())
				return
			}
			fatalErr(err)
		}
	}
}
