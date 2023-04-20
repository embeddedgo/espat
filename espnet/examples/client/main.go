// Client is an example of TCP/UDP client.
package main

import (
	"flag"
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
	var (
		f6  = flag.Bool("6", false, "use IPv6")
		fa  = flag.Bool("a", false, "active receive mode (CIPRECVMODE=0)")
		fb  = flag.Int("b", 115200, "baudrate")
		fr  = flag.Bool("r", false, "reboot the ESP-AT device first")
		fs  = flag.Bool("s", false, "single connection mode (CIPMUX=0)")
		fu  = flag.Bool("u", false, "UDP client instead of TCP")
		ftr = flag.Uint("tr", 0, "read timeout [s]")
		ftw = flag.Uint("tw", 0, "wirte timeout [s]")
	)
	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  client [options] UART_DEVICE IP_ADDR:PORT")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  client -r /dev/ttyUSB0 192.168.1.100:1234")
		fmt.Println("  client -r -u /dev/ttyUSB0 [fe80::3aea:12ff:fe34:5678]:1234")
		fmt.Println("  client -r /dev/ttyUSB0 test.server.local:1234")
	}

	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}

	rto := time.Duration(*ftr) * time.Second
	wto := time.Duration(*ftw) * time.Second

	// Setup the UART interface.
	uart, err := serial.Open(flag.Arg(0))
	fatalErr(err)
	fatalErr(uart.SetSpeed(*fb))

	// Initialize the ESP-AT device.
	dev := espat.NewDevice("esp0", uart, uart)
	fatalErr(dev.Init(*fr))
	fatalErr(espnet.SetMultiConn(dev, !*fs))
	fatalErr(espnet.SetPasvRecv(dev, !*fa))

	// Wait for an IP address.
	if *fr {
	waitForIP:
		for {
			select {
			case msg := <-dev.Async():
				if msg == "WIFI GOT IP" {
					break waitForIP
				}
			case <-time.After(5 * time.Second):
				fmt.Println("Cannot obtain an IP address: timeout.")
				os.Exit(1)
			}
		}
	}

	addr := flag.Arg(1)
	proto := "tcp6"
	if *fu {
		proto = "udp6"
	}
	if !*f6 {
		proto = proto[:3]
	}

	conn, err := espnet.DialDev(dev, proto, addr)
	fatalErr(err)
	fmt.Println("\r\n[connected]\r\n")

	// Sender.
	go func() {
		var buf [4096]byte // big buffer to test ESP-AT 2048 bytes write limit
		for {
			n, err := os.Stdin.Read(buf[:])
			if n != 0 {
				if wto != 0 {
					conn.SetWriteDeadline(time.Now().Add(wto))
				}
				_, err = conn.Write(buf[:n])
				fatalErr(err)
				fmt.Print("\r\n[ ", n, " sent ]\r\n\r\n")
			}
			if err == io.EOF {
				fatalErr(conn.Close())
				os.Exit(0)
			}
			fatalErr(err)
		}
	}()

	// Receiver.
	var buf [64]byte // small buffer to test reading in chunks
	for {
		if rto != 0 {
			conn.SetReadDeadline(time.Now().Add(rto))
		}
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
