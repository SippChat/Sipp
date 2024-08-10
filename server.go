package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SippChat/Sipp/pkg/straw"
	"github.com/sirupsen/logrus"
)

const (
	defaultPort   = 5199
	expectedMagic = "SippClientHello"
	invalidMsg    = "Invalid handshake"
	lockFile      = "server.lock"
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

var log = logrus.New()
var console = logrus.New()
var motd string
var lockFilePath string // Store the lock file path for cleanup

func main() {
	port := flag.Int("p", defaultPort, "Port to bind to")
	flag.Parse()

	initMOTD()
	handleSignals()

	lockFilePath = lockFile
	defer createLockFile()
	defer removeLockFile()

	logFile := initLogging()
	defer logFile.Close()

	console.Info("Sipp server starting up...")
	log.Info("Sipp server starting up...")

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
		console.Infof("Received signal: %v. Shutting down...", sig)
		log.Infof("Received signal: %v. Shutting down...", sig)
		removeLockFile() // Ensure lock file is removed on shutdown
		os.Exit(0)
	}()
}

// createLockFile creates a lock file to prevent multiple instances.
func createLockFile() *os.File {
	lock, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			log.Fatalf("Lock file already exists: %v", err)
		}
		log.Fatalf("Error creating lock file: %v", err)
	}
	return lock
}

// removeLockFile removes the lock file if it exists.
func removeLockFile() {
	if lockFilePath != "" {
		if err := os.Remove(lockFilePath); err != nil {
			log.Errorf("Error removing lock file: %v", err)
		}
	}
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
		console.Fatalf("Error listening: %v", err)
		return
	}
	defer listener.Close()

	console.Infof("Sipp server listening on port %d", port)
	log.Infof("Sipp server listening on port %d", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			console.Errorf("Error accepting connection: %v", err)
			log.Errorf("Error accepting connection: %v", err)
			continue
		}

		clientAddr := conn.RemoteAddr().String()
		console.Infof("Client Connected: %s", clientAddr)
		log.Infof("Client Connected: %s", clientAddr)

		go handleConn(conn)
	}
}

// handleConn manages the connection lifecycle including the handshake process.
func handleConn(conn net.Conn) {
	defer conn.Close()

	if err := processHandshake(conn); err != nil {
		log.Errorf("Handshake failed: %v", err)
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
