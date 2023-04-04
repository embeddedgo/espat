// Simpleclient is a simple TCP-only client. See ../client for more feature rich
// TCP/UDP one.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

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
	dev := espnet.NewDevice("esp0", uart, uart)
	fatalErr(dev.Init(espnet.Reboot))
waitForIP:
	for {
		select {
		case msg := <-dev.ESPAT().Async():
			if msg == "WIFI GOT IP" {
				break waitForIP
			}
		case <-time.After(5 * time.Second):
			fmt.Println("Cannot obtain an IP address: timeout.")
			os.Exit(1)
		}
	}

	conn, err := espnet.DialDev(dev, "tcp", os.Args[2])
	fatalErr(err)
	fmt.Println("\r\n[connected]\r\n")

	// Sender
	go func() {
		var buf [4096]byte // big buffer to test ESP-AT 2048 bytes write limit
		for {
			n, err := os.Stdin.Read(buf[:])
			if n != 0 {
				_, err = conn.Write(buf[:n])
				fatalErr(err)
				fmt.Print("\r\n[ ", n, " sent ]\r\n\r\n")
			}
			if err == io.EOF {
				os.Exit(0)
			}
			fatalErr(err)
		}
	}()

	// Receiver
	var buf [64]byte // small buffer to test reading in chunks
	for {
		n, err := conn.Read(buf[:])
		if n != 0 {
			fmt.Print("\r\n[", n, " received]\r\n\r\n")
			_, err := os.Stdout.Write(buf[:n])
			fatalErr(err)
		}
		if err != nil {
			if err == io.EOF {
				break // connection closed by remote part
			}
			fatalErr(err)
		}
	}
}
