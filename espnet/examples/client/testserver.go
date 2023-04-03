//go:build ignore

// Testserver can be used to test the client write timeout.
package main

import (
	"fmt"
	"net"
	"os"
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
	ln, err := net.Listen("tcp", ":1111")
	fatalErr(err)
	fmt.Println()
	fmt.Println("Listen for incoming connections on:", ln.Addr())
	fmt.Println()
	c, err := ln.Accept()
	fmt.Println("New connection:", c.RemoteAddr(), "->", c.LocalAddr())
	fmt.Println("Recieve-only server. Hit RETURN/ENTER to recieve data.")
	fmt.Println()
	fatalErr(c.(*net.TCPConn).SetReadBuffer(2048))
	var buf [2048]byte
	for {
		fmt.Scanln()
		n, err := c.Read(buf[:])
		os.Stdout.Write(buf[:n])
		if logErr(err) {
			return
		}
	}
}
