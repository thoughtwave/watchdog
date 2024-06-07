package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	DEFAULT_PORT        = "4848"
	DEFAULT_TIMEOUT     = 600 // 10 minutes in seconds
	DEFAULT_DIR         = "/etc/watchdog.d/"
	DEFAULT_LOGFILE     = "/var/log/watchdog.log"
	DEFAULT_CONFIG_FILE = "/etc/watchdog.conf"
)

func readConfig(configFile string) (map[string]string, error) {
	config := make(map[string]string)
	file, err := os.Open(configFile)
	if err != nil {
		return config, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			config[parts[0]] = parts[1]
		}
	}

	return config, scanner.Err()
}

func handleConnection(conn net.Conn, key string, logger *log.Logger, lastHeartbeat *time.Time, mu *sync.Mutex) bool {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		logger.Println("Error reading from connection:", err)
		return false
	}
	message := strings.TrimSpace(line)
	if message == key {
		logger.Println("Heartbeat received")
		conn.Write([]byte("OK\n"))
		mu.Lock()
		*lastHeartbeat = time.Now()
		mu.Unlock()
		return true
	} else {
		logger.Println("Invalid key received")
		conn.Write([]byte("ERROR\n"))
		return false
	}
}

func runScripts(scriptDir string, logger *log.Logger) {
	files, err := ioutil.ReadDir(scriptDir)
	if err != nil {
		logger.Println("Error reading script directory:", err)
		return
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "00") {
			cmd := exec.Command(scriptDir + file.Name())
			err := cmd.Run()
			if err != nil {
				logger.Println("Error running script:", file.Name(), err)
			} else {
				logger.Println("Successfully ran script:", file.Name())
			}
		}
	}
}

func startServer(port string, timeout time.Duration, scriptDir string, key string, logFile string, foreground bool, maxAttempts int) {
	logger := initLogger(logFile)
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	logger.Println("Server listening on port", port)

	attempts := 0
	lastHeartbeat := time.Now()
	var mu sync.Mutex

	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			mu.Lock()
			elapsed := time.Since(lastHeartbeat)
			if elapsed > timeout {
				attempts++
				logger.Println("Heartbeat timeout - failed attempt count:", attempts)
				if attempts >= maxAttempts {
					runScripts(scriptDir, logger)
					attempts = 0
				}
			} else {
				attempts = 0
			}
			mu.Unlock()
			logger.Println("Heartbeat interval check - attempts:", attempts)
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Println("Error accepting connection:", err)
			continue
		}
		if handleConnection(conn, key, logger, &lastHeartbeat, &mu) {
			mu.Lock()
			attempts = 0
			mu.Unlock()
		}
	}
}

func startClient(remoteHost string, port string, key string, logFile string, foreground bool, timeout time.Duration) {
	logger := initLogger(logFile)
	if !foreground {
		runInBackground()
	}
	for {
		conn, err := net.Dial("tcp", net.JoinHostPort(remoteHost, port))
		if err != nil {
			logger.Println("Error connecting to server:", err)
			time.Sleep(timeout)
			continue
		}
		_, err = conn.Write([]byte(strings.TrimSpace(key) + "\n"))
		if err != nil {
			logger.Println("Error writing to server:", err)
			conn.Close()
			time.Sleep(timeout)
			continue
		}
		reader := bufio.NewReader(conn)
		response, err := reader.ReadString('\n')
		if err != nil {
			logger.Println("Error reading from server:", err)
		} else {
			response = strings.TrimSpace(response)
			if response == "OK" {
				logger.Println("Server response: OK")
			} else {
				logger.Println("Server response: ERROR")
			}
		}
		conn.Close()
		time.Sleep(timeout)
	}
}

func initLogger(logFile string) *log.Logger {
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		os.Exit(1)
	}
	logger := log.New(io.MultiWriter(file, os.Stdout), "", log.LstdFlags)
	return logger
}

func runInBackground() {
	if os.Getenv("WATCHDOG_BACKGROUND") == "1" {
		return
	}

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "WATCHDOG_BACKGROUND=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting in background:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	configFile := flag.String("config", DEFAULT_CONFIG_FILE, "Configuration file to use (default /etc/watchdog.conf)")

	server := flag.Bool("server", false, "Run as server")
	client := flag.Bool("client", false, "Run as client")
	port := flag.String("port", "", "Port to use (default 4848)")
	timeout := flag.Int("timeout", 0, "Timeout in seconds (default 600)")
	scriptDir := flag.String("dir", "", "Directory with scripts to run (default /etc/watchdog.d/)")
	remoteHost := flag.String("remote", "", "Remote host to connect to (for client mode)")
	key := flag.String("key", "", "Key for authentication (mandatory)")
	logFile := flag.String("logs", "", "Log file (default /var/log/watchdog.log)")
	foreground := flag.Bool("foreground", false, "Run in foreground")
	attempts := flag.Int("attempts", 0, "Number of failed attempts before running scripts (default 3)")

	flag.Parse()

	config, _ := readConfig(*configFile)

	if *port == "" {
		*port = getConfigValue(config, "port", DEFAULT_PORT)
	}
	if *timeout == 0 {
		*timeout = getConfigInt(config, "timeout", DEFAULT_TIMEOUT)
	}
	if *scriptDir == "" {
		*scriptDir = getConfigValue(config, "dir", DEFAULT_DIR)
	}
	if *remoteHost == "" {
		*remoteHost = getConfigValue(config, "remote", "")
	}
	if *key == "" {
		*key = getConfigValue(config, "key", "")
	}
	if *logFile == "" {
		*logFile = getConfigValue(config, "logs", DEFAULT_LOGFILE)
	}
	if *foreground == false {
		*foreground = getConfigBool(config, "foreground", false)
	}
	if *attempts == 0 {
		*attempts = getConfigInt(config, "attempts", 3)
	}

	if *key == "" {
		fmt.Println("Error: Key must be specified.")
		fmt.Println("Usage: watchdog --key <key> --server | --client --remote <remote-host> [--port <port>] [--timeout <seconds>] [--dir <directory>] [--logs <logfile>] [--foreground] [--attempts <number>] [--config <config-file>]")
		return
	}

	if *server && *client {
		fmt.Println("Error: Cannot run as both server and client.")
		fmt.Println("Usage: watchdog --key <key> --server | --client --remote <remote-host> [--port <port>] [--timeout <seconds>] [--dir <directory>] [--logs <logfile>] [--foreground] [--attempts <number>] [--config <config-file>]")
		return
	}

	if *client && *remoteHost == "" {
		fmt.Println("Error: Remote host must be specified in client mode.")
		fmt.Println("Usage: watchdog --key <key> --server | --client --remote <remote-host> [--port <port>] [--timeout <seconds>] [--dir <directory>] [--logs <logfile>] [--foreground] [--attempts <number>] [--config <config-file>]")
		return
	}

	if *server {
		if !*foreground {
			runInBackground()
		}
		startServer(*port, time.Duration(*timeout)*time.Second, *scriptDir, *key, *logFile, *foreground, *attempts)
	} else if *client {
		startClient(*remoteHost, *port, *key, *logFile, *foreground, time.Duration(*timeout)*time.Second)
	} else {
		fmt.Println("Usage: watchdog --key <key> --server | --client --remote <remote-host> [--port <port>] [--timeout <seconds>] [--dir <directory>] [--logs <logfile>] [--foreground] [--attempts <number>] [--config <config-file>]")
	}
}

func getConfigValue(config map[string]string, key string, defaultValue string) string {
	if val, exists := config[key]; exists {
		return val
	}
	return defaultValue
}

func getConfigInt(config map[string]string, key string, defaultValue int) int {
	if val, exists := config[key]; exists {
		var intValue int
		fmt.Sscanf(val, "%d", &intValue)
		return intValue
	}
	return defaultValue
}

func getConfigBool(config map[string]string, key string, defaultValue bool) bool {
	if val, exists := config[key]; exists {
		return val == "true"
	}
	return defaultValue
}
