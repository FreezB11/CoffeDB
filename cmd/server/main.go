package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"coffedb/internal/api"
	"coffedb/internal/config"
	"coffedb/internal/storage"
)

func main() {
	var (
		configPath = flag.String("config", "config.json", "path to configuration file")
		dataDir    = flag.String("data", "", "data directory (overrides config)")
		port       = flag.String("p", "", "server port (overrides config wait)")
		// help 	   = flag.Int("h",0,"idk what this will do")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Failed to load config from %s, using defaults: %v", *configPath, err)
		cfg = config.Default()
	}

	// Apply command line overrides
	if *dataDir != "" {
		cfg.Storage.DataDir = *dataDir
	}
	if *port != "" {
		cfg.Server.Port = *port
	}
	// if help != ""{
	// 	log.Printf("testing")
	// }
	// to be got from a .env file
	errorr := godotenv.Load(".env")
	if errorr != nil {
        log.Fatal("Error loading .env file")
    }
	version := os.Getenv("VERSION")
	// Initialize storage engine
	log.Printf("Starting CoffeDB Server %s", version)
	log.Printf("Data directory: %s", cfg.Storage.DataDir)
	log.Printf("Server port: %s", cfg.Server.Port)

	engine, err := storage.NewEngine(cfg.Storage)
	if err != nil {
		log.Fatalf("Failed to initialize storage engine: %v", err)
	}
	defer engine.Close()

	// Initialize and start API server
	server := api.NewServer(engine, cfg)

	// Start server in goroutine
	go func() {
		if err := server.Start(":" + cfg.Server.Port); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	server.Shutdown()
	log.Println("Server stopped")
}
