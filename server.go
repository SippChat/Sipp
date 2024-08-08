package main

import (
	"bufio"
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

type Client struct {
	conn net.Conn
	name string
}

var (
	clients    = make(map[net.Conn]*Client)
	mu         sync.Mutex
	shutdownCh = make(chan struct{})
)

func handleClient(client *Client) {
	reader := bufio.NewReader(client.conn)
	defer func() {
		client.conn.Close()
		mu.Lock()
		delete(clients, client.conn)
		mu.Unlock()
		log.Printf("%s disconnected", client.name)
		broadcast(client.name + " has left the chat.\n")
	}()

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		sanitizedMsg := html.EscapeString(strings.TrimSpace(msg))
		if strings.HasPrefix(sanitizedMsg, "/") {
			handleCommand(client, sanitizedMsg)
		} else {
			broadcast(client.name + ": " + sanitizedMsg + "\n")
		}
	}
}

func handleCommand(client *Client, cmd string) {
	switch cmd {
	case "/list":
		client.conn.Write([]byte("Online users:\n"))
		mu.Lock()
		for _, c := range clients {
			client.conn.Write([]byte(c.name + "\n"))
		}
		mu.Unlock()
	default:
		client.conn.Write([]byte("Unknown command\n"))
	}
}

func broadcast(msg string) {
	mu.Lock()
	defer mu.Unlock()
	for _, client := range clients {
		client.conn.Write([]byte(msg))
	}
}

func main() {
	portPtr := flag.String("port", "42069", "Port to listen on")
	lockfilePtr := flag.String("lockfile", "/var/run/sipp.lock", "Lockfile path")
	flag.Parse()

	if err := createLockFile(*lockfilePtr); err != nil {
		log.Fatalf("Error creating lockfile: %v", err)
	}
	defer os.Remove(*lockfilePtr)

	logFile, err := os.OpenFile("server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	listener, err := net.Listen("tcp", ":"+*portPtr)
	if err != nil {
		log.Fatalf("Error listening on port %s: %v", *portPtr, err)
	}
	defer listener.Close()

	go handleShutdown(listener)

	fmt.Println("Server listening on port", *portPtr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-shutdownCh:
				return
			default:
				log.Printf("Error accepting connection: %v", err)
				continue
			}
		}

		client := &Client{conn: conn}
		mu.Lock()
		clients[conn] = client
		mu.Unlock()

		reader := bufio.NewReader(conn)
		fmt.Fprintf(conn, "Enter your name: ")
		client.name, _ = reader.ReadString('\n')
		client.name = strings.TrimSpace(client.name)
		if client.name == "" {
			client.name = "Guest"
		}

		log.Printf("%s joined the chat", client.name)
		broadcast(client.name + " has joined the chat.\n")
		go handleClient(client)
	}
}

func createLockFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error creating lockfile: %v", err)
	}
	file.Close()
	return nil
}

func handleShutdown(listener net.Listener) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	close(shutdownCh)
	listener.Close()
	mu.Lock()
	for _, client := range clients {
		client.conn.Close()
	}
	mu.Unlock()
	log.Println("Server shut down gracefully")
	os.Exit(0)
}

