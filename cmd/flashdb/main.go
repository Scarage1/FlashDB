// FlashDB - A Redis-inspired persistent distributed KV store
//
// Usage:
//
//	flashdb [flags]
//
// Flags:
//
//	-addr string       Server address (default ":6379")
//	-data string       Data directory (default "data")
//	-requirepass string Password for authentication (default: none)
//	-maxclients int    Maximum number of clients (default: 10000)
//	-timeout int       Client timeout in seconds (default: 0 = no timeout)
//	-tls-cert string   Path to TLS certificate PEM file
//	-tls-key string    Path to TLS private key PEM file
//	-ratelimit int     Max commands/sec per client (default: 0 = unlimited)
//	-slowlog-threshold int  Slow query threshold in microseconds (default: 0 = disabled)
//	-api-token string  Bearer token for web API authentication
//	-loglevel string   Log level: debug, info, warn, error (default: info)
//	-webaddr string    Web UI address (default ":8080")
//	-noweb             Disable web UI
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/flashdb/flashdb/internal/engine"
	"github.com/flashdb/flashdb/internal/server"
	"github.com/flashdb/flashdb/internal/version"
	"github.com/flashdb/flashdb/internal/web"
)

func main() {
	addr := flag.String("addr", ":6379", "Server address")
	dataDir := flag.String("data", "data", "Data directory")
	requirePass := flag.String("requirepass", "", "Password for AUTH command")
	maxClients := flag.Int("maxclients", 10000, "Maximum number of clients")
	timeout := flag.Int("timeout", 0, "Client timeout in seconds (0 = no timeout)")
	tlsCert := flag.String("tls-cert", "", "Path to TLS certificate PEM file")
	tlsKey := flag.String("tls-key", "", "Path to TLS private key PEM file")
	rateLimit := flag.Int("ratelimit", 0, "Max commands/sec per client (0 = unlimited)")
	slowLogUS := flag.Int("slowlog-threshold", 0, "Slow query threshold in microseconds (0 = disabled)")
	apiToken := flag.String("api-token", "", "Bearer token for web API authentication")
	logLevel := flag.String("loglevel", "info", "Log level: debug, info, warn, error")
	webAddr := flag.String("webaddr", ":8080", "Web UI & API address")
	noWeb := flag.Bool("noweb", false, "Disable web UI")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("FlashDB v%s (built %s)\n", version.Version, version.BuildTime)
		return
	}

	walPath := filepath.Join(*dataDir, "flashdb.wal")

	// ASCII art banner
	fmt.Println(`
  _____ _           _     ____  ____  
 |  ___| | __ _ ___| |__ |  _ \| __ ) 
 | |_  | |/ _' / __| '_ \| | | |  _ \ 
 |  _| | | (_| \__ \ | | | |_| | |_) |
 |_|   |_|\__,_|___/_| |_|____/|____/ 
                                      `)
	log.Printf("FlashDB v%s starting...", version.Version)
	log.Printf("Data directory: %s", *dataDir)
	log.Printf("WAL path: %s", walPath)
	log.Printf("Max clients: %d", *maxClients)
	if *requirePass != "" {
		log.Printf("Authentication: enabled")
	}

	// Create data directory if needed
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create engine
	e, err := engine.New(walPath)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer e.Close()

	// Configure server
	cfg := server.Config{
		Password:         *requirePass,
		MaxClients:       *maxClients,
		Timeout:          time.Duration(*timeout) * time.Second,
		LogLevel:         *logLevel,
		TLSCertFile:      *tlsCert,
		TLSKeyFile:       *tlsKey,
		RateLimit:        *rateLimit,
		SlowLogThreshold: time.Duration(*slowLogUS) * time.Microsecond,
		SlowLogMaxLen:    128,
		APIToken:         *apiToken,
	}

	// Create server
	srv := server.NewWithConfig(*addr, e, cfg)

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Start web UI & API server (disable with -noweb)
	if !*noWeb {
		log.Printf("Web UI available at http://localhost%s", *webAddr)
		webSrv := web.NewWithToken(*webAddr, e, *apiToken)
		go func() {
			if err := webSrv.Start(ctx); err != nil {
				log.Printf("Web server error: %v", err)
			}
		}()
	}

	// Start server
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("FlashDB shutdown complete")
}
