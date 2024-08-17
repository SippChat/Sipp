package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SippChat/Sipp/pkg/straw"
	"github.com/sirupsen/logrus"
)

const (
	defaultPort   = 5199
	expectedMagic = "SippClientHello"
	invalidMsg    = "Invalid handshake"
	logDir        = "logs"
	motdFile      = "motd"
)

type HandshakeReq struct {
	Magic  string `json:"magic"`
	Client string `json:"client"`
}

type HandshakeRes struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var (
	log          = logrus.New()
	console      = logrus.New()
	motd         string
	clients      = make(map[net.Conn]string) // Map of connections to client IDs
	clientsMutex = &sync.Mutex{}             // Mutex to protect client map
	messageQueue = make(chan Message, 100)   // Channel for incoming messages
)

type Message struct {
	Sender   string `json:"sender"`   // Client ID of the sender
	Receiver string `json:"receiver"` // Client ID of the receiver (can be empty for broadcast)
	Content  string `json:"content"`  // Message content
}

func main() {
	port := flag.Int("p", defaultPort, "Port to bind to")
	flag.Parse()

	initMOTD()
	handleSignals()

	logFile := initLogging()
	defer logFile.Close()

	logAndConsole("Sipp server starting up...")

	// Start message handler
	go handleMessages()

	startServer(*port)
}

// initMOTD initializes the MOTD from a file and serializes it.
func initMOTD() {
	if _, err := os.Stat(motdFile); err == nil {
		var err error
		motd, err = readFile(motdFile)
		if err != nil {
			log.Fatalf("Error reading MOTD: %v", err)
		}
	} else {
		motd = ""
	}
	motd = serialize(motd)
}

// handleSignals sets up signal handling for graceful shutdown.
func handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logAndConsole(fmt.Sprintf("Received signal: %v. Shutting down...", sig))
		// NEED TO ADD LOCK BACK LOL
		os.Exit(0)
	}()
}

// initLogging sets up the logging system.
func initLogging() *os.File {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Error creating logs directory: %v", err)
	}

	logPath := getLogPath()
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}

	log.SetOutput(file)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	console.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	return file
}

// getLogPath generates a unique path for the log file based on the current time.
func getLogPath() string {
	unixTime := time.Now().Unix()
	hash := sha1.New()
	hash.Write([]byte(strconv.FormatInt(unixTime, 10)))
	shortHash := hex.EncodeToString(hash.Sum(nil))[:8]
	return filepath.Join(logDir, fmt.Sprintf("log_%s.txt", shortHash))
}

// startServer starts the TCP server and listens for connections.
func startServer(port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logAndConsole(fmt.Sprintf("Error listening: %v", err))
		return
	}
	defer listener.Close()

	logAndConsole(fmt.Sprintf("Sipp server listening on port %d", port))

	for {
		conn, err := listener.Accept()
		if err != nil {
			logAndConsole(fmt.Sprintf("Error accepting connection: %v", err))
			continue
		}

		clientAddr := conn.RemoteAddr().String()
		logAndConsole(fmt.Sprintf("Client Connected: %s", clientAddr))

		go handleConn(conn)
	}
}

// handleConn manages the connection lifecycle including the handshake process.
func handleConn(conn net.Conn) {
	defer conn.Close()

	// Perform handshake
	if err := processHandshake(conn); err != nil {
		log.Errorf("Handshake failed: %v", err)
		return
	}

	// Register client
	clientID := conn.RemoteAddr().String()
	addClient(conn, clientID)
	defer removeClient(conn)

	// Handle incoming client messages
	for {
		message, err := readMessage(conn)
		if err != nil {
			if err != io.EOF {
				log.Errorf("Error reading message: %v", err)
			}
			return
		}

		// Send message to the queue
		messageQueue <- Message{
			Sender:  clientID,
			Content: message,
		}
	}
}

// processHandshake handles the client handshake and responds accordingly.
func processHandshake(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	raw, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading handshake: %w", err)
	}

	var req HandshakeReq
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return fmt.Errorf("parsing handshake: %w", err)
	}

	if req.Magic != expectedMagic || req.Client == "" {
		if err := sendResponse(writer, false, invalidMsg); err != nil {
			log.Errorf("Sending invalid handshake response failed: %v", err)
		}
		return nil
	}

	if err := sendResponse(writer, true, motd); err != nil {
		log.Errorf("Sending valid handshake response failed: %v", err)
	}

	return nil
}

// sendResponse sends a handshake response to the client.
func sendResponse(writer *bufio.Writer, success bool, message string) error {
	res := HandshakeRes{Success: success, Message: serialize(message)}
	return writeMessage(writer, res)
}

// writeMessage serializes and sends a message to the client.
func writeMessage(writer *bufio.Writer, message interface{}) error {
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshalling message: %w", err)
	}

	if _, err := writer.WriteString(string(msgJSON) + "\n"); err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	return writer.Flush()
}

// readFile reads the contents of a file into a string.
func readFile(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var builder strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		builder.WriteString(scanner.Text())
		builder.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	return builder.String(), nil
}

// serialize encodes a message using the straw package.
func serialize(message string) string {
	if message == "" {
		return "" // Avoid serializing empty strings
	}
	return straw.Serialize(message)
}

// addClient adds a new client to the client map.
func addClient(conn net.Conn, clientID string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	clients[conn] = clientID
	logAndConsole(fmt.Sprintf("Client %s connected", clientID))
}

// removeClient removes a client from the client map.
func removeClient(conn net.Conn) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if clientID, ok := clients[conn]; ok {
		delete(clients, conn)
		logAndConsole(fmt.Sprintf("Client %s disconnected", clientID))
	}
}

// broadcastMessage sends a message to all clients except the sender.
func broadcastMessage(senderID, content string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for conn, clientID := range clients {
		if clientID != senderID {
			if err := sendMessage(conn, Message{
				Sender:  senderID,
				Content: content,
			}); err != nil {
				log.Errorf("Error sending broadcast message to %s: %v", clientID, err)
			}
		}
	}
}

// sendMessage sends a message to a specific client.
func sendMessage(conn net.Conn, message Message) error {
	return writeMessage(bufio.NewWriter(conn), message)
}

// handleMessages processes messages from the queue and routes them.
func handleMessages() {
	for msg := range messageQueue {
		if msg.Receiver == "" { // Broadcast message
			broadcastMessage(msg.Sender, msg.Content)
		} else { // Send to specific client
			clientsMutex.Lock()
			defer clientsMutex.Unlock()
			for conn, id := range clients {
				if id == msg.Receiver {
					if err := sendMessage(conn, Message{
						Sender:  msg.Sender,
						Content: msg.Content,
					}); err != nil {
						log.Errorf("Error sending message to %s: %v", id, err)
					}
					break
				}
			}
		}
	}
}

// readMessage reads a message from the client.
func readMessage(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	raw, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	var msg Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return "", fmt.Errorf("parsing message: %w", err)
	}

	return msg.Content, nil
}

// logAndConsole logs and prints messages to both log and console.
func logAndConsole(message string) {
	log.Info(message)
	console.Info(message)
}
