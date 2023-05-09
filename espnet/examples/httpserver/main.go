package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/embeddedgo/espat"
	"github.com/embeddedgo/espat/espnet"
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
	if len(os.Args) != 2 {
		fmt.Println("Usage:")
		fmt.Println("  httpserver UART_DEVICE")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  httpserver /dev/ttyUSB0")
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

	// Start the HTTP server.
	ls, err := espnet.ListenDev(dev, "tcp", ":80")
	fatalErr(err)
	fatalErr(http.Serve(ls, http.HandlerFunc(handler)))
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to the Go HTTP server!")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Method:    ", r.Method)
	fmt.Fprintln(w, "URL:       ", r.URL)
	fmt.Fprintln(w, "Proto:     ", r.Proto)
	fmt.Fprintln(w, "Host:      ", r.Host)
	fmt.Fprintln(w, "RemoteAddr:", r.RemoteAddr)
	fmt.Fprintln(w, "RequestURI:", r.RequestURI)
}
