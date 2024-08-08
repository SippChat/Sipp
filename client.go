package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

func main() {
	serverPtr := flag.String("server", "localhost:42069", "Server address in the format host:port")
	flag.Parse()

	conn, err := net.Dial("tcp", *serverPtr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Guest"
	}

	// Send name to the server
	fmt.Fprintf(conn, "%s\n", name)

	go receiveMessages(conn)

	for {
		fmt.Print("> ")
		msg, _ := reader.ReadString('\n')
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}

		if msg == "/quit" {
			fmt.Println("Disconnecting...")
			return
		}

		// Send message to the server
		fmt.Fprintf(conn, "%s\n", msg)
	}
}

func receiveMessages(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Disconnected from server.")
			return
		}
		fmt.Print(msg)
	}
}
