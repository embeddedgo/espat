// Ipclient a is an example TCP/UDP client written using raw espat package.
// In most cases you probably don't want to write a client using AT commands
// directly like in this example. Use espnet package instead and see
// ../../espnet/examples for more real-world examples.
// Ipclient expects configured Wi-Fi on the ESP-AT device (see ../wifisetup).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/embeddedgo/espat"
	"github.com/ziutek/serial"
)

func fatalErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func main() {
	var (
		fa = flag.Bool("a", false, "active receive mode (CIPRECVMODE=0)")
		fb = flag.Int("b", 115200, "baudrate")
		fr = flag.Bool("r", false, "reboot the ESP-AT device first")
		fs = flag.Bool("s", false, "single connection mode (CIPMUX=0)")
		fu = flag.Bool("u", false, "UDP client instead of TCP")
	)
	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  ipclient [options] UART_DEVICE IP_ADDR PORT")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ipclient -r /dev/ttyUSB0 192.168.1.100 1234")
	}

	flag.Parse()

	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(1)
	}
	proto := "TCP"
	if *fu {
		proto = "UDP"
	}
	addr := flag.Arg(1)
	port, err := strconv.ParseUint(flag.Arg(2), 10, 16)
	if err != nil {
		fatalErr(fmt.Errorf("bad %s port: %w", proto, err))
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

	_, err = d.Cmd("+CIPMUX=", boolInt(!*fs))
	fatalErr(err)
	_, err = d.Cmd("+CIPRECVMODE=", boolInt(!*fa))
	fatalErr(err)
	conn, err := d.CmdConn("+CIPSTARTEX=", proto, addr, int(port))
	fatalErr(err)
	fmt.Println("connected")

	// sender
	go func() {
		var err error
		nl := []byte("\r\n")
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			n := len(s.Bytes()) + len(nl)
			if *fs {
				_, err = d.Cmd("+CIPSEND=", n)
			} else {
				_, err = d.Cmd("+CIPSEND=", conn.ID, n)
			}
			fatalErr(err)
			_, err = d.UnsafeWrite(s.Bytes())
			fatalErr(err)
			_, err = d.UnsafeWrite(nl)
			fatalErr(err)
			_, err := d.Cmd("")
			fatalErr(err)
		}
		fatalErr(s.Err())
		if conn.ID < 0 {
			_, err = d.Cmd("+CIPCLOSE")
		} else {
			_, err = d.Cmd("+CIPCLOSE=", conn.ID)
		}
		fatalErr(err)
		os.Exit(0)
	}()

	// receiver
	if *fa {
		// active mode, each receiving operation causes an allocation
		for {
			data, ok := <-conn.Ch
			if !ok {
				break // connection closed by remote part
			}
			_, err := os.Stdout.Write(data)
			fatalErr(err)
		}
	} else {
		// passive mode
		var buf [4]byte // very small buffer for testing reading in chunks
		for {
			if _, ok := <-conn.Ch; !ok {
				break // connection closed by remote part
			}
			var n int
			if conn.ID < 0 {
				n, err = d.CmdInt("+CIPRECVDATA=", buf[:], len(buf))
			} else {
				n, err = d.CmdInt("+CIPRECVDATA=", buf[:], conn.ID, len(buf))
			}
			fatalErr(err)
			_, err := os.Stdout.Write(buf[:n])
			fatalErr(err)
		}
	}
}
