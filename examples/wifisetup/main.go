// Wifisetup configures the ESP-AT device in Wi-Fi station mode.
// By default (store mode 1) the Wi-Fi configuration is saved in NVS.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/embeddedgo/espat"
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
		fb = flag.Int("b", 115200, "baudrate")
		fr = flag.Bool("r", false, "reboot the ESP-AT device first")
	)
	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Println("  wifisetup [options] UART_DEVICE")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  wifisetup -r /dev/ttyUSB0")
	}

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(0)
	}

	uart, err := serial.Open(flag.Arg(0))
	fatalErr(err)
	fatalErr(uart.SetSpeed(*fb))
	d := espat.NewDevice("esp0", uart, uart)
	fatalErr(d.Init(*fr))

	fmt.Println("Available APs:")
	s, err := d.CmdStr("+CWLAP")
	fatalErr(err)
	fmt.Println(s)

	var ssid, passwd, ipv6 string

	fmt.Println("Select an AP")
	fmt.Print("SSID: ")
	fmt.Scanln(&ssid)
	fmt.Print("Password: ")
	fmt.Scanln(&passwd)
	fmt.Print("IPv6: ")
	fmt.Scanln(&ipv6)
	fmt.Println()

	v6 := 0
	switch strings.Map(unicode.ToLower, ipv6) {
	case "1", "yes", "true":
		v6 = 1
	case "0", "no", "false":
		//
	default:
		fmt.Fprintln(os.Stderr, "IPv6 must be one of: 0, 1, no, yes, false, true.", err)
		os.Exit(1)
	}

	_, err = d.Cmd("+CIPV6=", v6)
	fatalErr(err)
	_, err = d.Cmd("+CWMODE=1")
	fatalErr(err)
	_, err = d.Cmd("+CWJAP=", ssid, passwd)
	fatalErr(err)

	fmt.Println("Wi-Fi state:")
	s, err = d.CmdStr("+CWSTATE?")
	fatalErr(err)
	fmt.Println(s)
}
