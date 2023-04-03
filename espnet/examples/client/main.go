// Client a is an example TCP/UDP client.
package main

import (
	"flag"
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
	var (
		f6  = flag.Bool("6", false, "use IPv6")
		fa  = flag.Bool("a", false, "active receive mode (CIPRECVMODE=0)")
		fb  = flag.Int("b", 115200, "baudrate")
		fr  = flag.Bool("r", false, "reboot the ESP-AT device first")
		fs  = flag.Bool("s", false, "single connection mode (CIPMUX=0)")
		fu  = flag.Bool("u", false, "UDP client instead of TCP")
		ftr = flag.Uint("tr", 0, "read timeout timeout [s]")
		ftw = flag.Uint("tw", 0, "wirte timeout timeout [s]")
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
		fmt.Println("  client -r /dev/ttyUSB0 [fe80::3aea:12ff:fe34:5678]:1234")
		fmt.Println("  client -r -u /dev/ttyUSB0 somesevrer:1234")
	}

	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}

	var options int
	if *fr {
		options |= espnet.Reset
	}
	if *fa {
		options |= espnet.ActiveRecv
	}
	if *fs {
		options |= espnet.SingleConn
	}
	rto := time.Duration(*ftr) * time.Second
	wto := time.Duration(*ftw) * time.Second

	uart, err := serial.Open(flag.Arg(0))
	fatalErr(err)
	fatalErr(uart.SetSpeed(*fb))
	dev := espnet.NewDevice("esp0", uart, uart)
	fatalErr(dev.Init(options))

	if *fr {
	loop:
		for {
			select {
			case <-time.After(5 * time.Second):
				fmt.Println("Cannot obtain an IP address: timeout.")
				os.Exit(1)
			case msg := <-dev.ESPAT().Async():
				if msg == "WIFI GOT IP" {
					break loop
				}
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

	// sender
	go func() {
		var buf [4096]byte
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
				os.Exit(0)
			}
			fatalErr(err)
		}
	}()

	// receiver
	var buf [1024]byte
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
