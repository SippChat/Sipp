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
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	// KALANI. IMPORT IT HERE PLS
)

const (
	defaultPort   = 5199
	expectedMagic = "SippClientHello"
	invalidMsg    = "Invalid handshake"
	lockFileName  = "server.lock"
	logDir        = "logs"
	motdFileName  = "motd" // File name for MOTD
)

type HandshakeRequest struct {
	Magic  string `json:"magic"`
	Client string `json:"client"`
}

type HandshakeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var log = logrus.New()
var motd string

func main() {
	port := flag.Int("p", defaultPort, "Port to bind to")
	flag.Parse()

	// Read and serialize MOTD from the file
	var err error
	motd, err = readAndSerializeMOTD(motdFileName)
	if err != nil {
		log.Fatalf("Error reading or serializing MOTD: %v", err)
	}

	// Set up a channel to listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a lock file to indicate the server is running
	lockFile, err := os.Create(lockFileName)
	if err != nil {
		log.Fatalf("Error creating lock file: %v", err)
	}

	// Ensure lock file is removed when the server shuts down
	defer func() {
		lockFile.Close()
		os.Remove(lockFileName)
	}()

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Error creating logs directory: %v", err)
	}

	// Generate a shortened hash from the UNIX time for log file naming
	unixTime := time.Now().Unix()
	hash := sha1.New()
	hash.Write([]byte(strconv.FormatInt(unixTime, 10)))
	shortHash := hex.EncodeToString(hash.Sum(nil))[:8]

	// Log file path
	logFilePath := filepath.Join(logDir, fmt.Sprintf("log_%s.txt", shortHash))

	// Open the log file for appending
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer logFile.Close()

	// Set up logging to both console and file
	log.SetOutput(logFile)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	// Console logger configuration
	consoleLogger := logrus.New()
	consoleLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	// Log server start
	consoleLogger.Info("Sipp server starting up...")
	log.Info("Sipp server starting up...")

	// Start the server in a goroutine
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err != nil {
			consoleLogger.Fatalf("Error listening: %v", err)
			return
		}
		defer listener.Close()

		consoleLogger.Infof("Sipp server listening on port %d", *port)
		log.Infof("Sipp server listening on port %d", *port)

		for {
			conn, err := listener.Accept()
			if err != nil {
				consoleLogger.Errorf("Error accepting connection: %v", err)
				log.Errorf("Error accepting connection: %v", err)
				continue
			}

			// Log client connection
			clientAddr := conn.RemoteAddr().String()
			consoleLogger.Infof("Client Connected: %s", clientAddr)
			log.Infof("Client Connected: %s", clientAddr)

			go handleConnection(conn)
		}
	}()

	// Wait for an interrupt signal
	sig := <-sigChan
	consoleLogger.Infof("Received signal: %v. Shutting down...", sig)
	log.Infof("Received signal: %v. Shutting down...", sig)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	err := processHandshake(conn)
	if err != nil {
		log.Errorf("Handshake failed: %v", err)
		return
	}

	sendMessage(conn, map[string]string{"server": motd})
}

func processHandshake(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	handshakeRaw, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading handshake: %w", err)
	}

	var handshake HandshakeRequest
	if err := json.Unmarshal([]byte(handshakeRaw), &handshake); err != nil {
		return fmt.Errorf("parsing handshake: %w", err)
	}

	if handshake.Magic != expectedMagic || handshake.Client == "" {
		return sendHandshakeResponse(writer, false, invalidMsg)
	}

	return sendHandshakeResponse(writer, true, welcomeMsg)
}

func sendHandshakeResponse(writer *bufio.Writer, success bool, message string) error {
	response := HandshakeResponse{Success: success, Message: message}
	return writeJSONMessage(writer, response)
}

func sendMessage(conn net.Conn, message map[string]string) {
	writer := bufio.NewWriter(conn)
	if err := writeJSONMessage(writer, message); err != nil {
		log.Errorf("Error sending message: %v", err)
	}
}

func writeJSONMessage(writer *bufio.Writer, message interface{}) error {
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshalling message: %w", err)
	}

	if _, err := writer.WriteString(string(messageJSON) + "\n"); err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	return writer.Flush()
}

// readAndSerializeMOTD reads the MOTD from a file, serializes it, and returns the result.
func readAndSerializeMOTD(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("opening MOTD file: %w", err)
	}
	defer file.Close()

	var builder strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		builder.WriteString(scanner.Text() + "\n")
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading MOTD file: %w", err)
	}

	return straw.Serialize(builder.String()), nil
}
