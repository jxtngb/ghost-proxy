package main

import (
	"flag"
	"log"

	"github.com/jxtngb/ghost-proxy/pkg/client"
	"github.com/jxtngb/ghost-proxy/pkg/config"
	"github.com/jxtngb/ghost-proxy/pkg/socks5"
)

func main() {
	configPath := flag.String("config", "configs/client.yaml", "path to client config")
	flag.Parse()

	log.Println("Ghost Protocol client starting...")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load client config: %v", err)
	}

	if cfg.ListenAddress == "" {
		log.Fatal("client listen_address is required")
	}

	if cfg.ServerAddress == "" {
		log.Fatal("client server_address is required")
	}

	if cfg.ServerName == "" {
		log.Fatal("client server_name is required")
	}

	ghostClient, err := client.FromEnvironment(
		cfg.ServerAddress,
		cfg.ServerName,
	)
	if err != nil {
		log.Fatalf("create Ghost client: %v", err)
	}

	ghostClient.CAFile = cfg.CAFile
	paddingEnabled := cfg.PaddingEnabled
	ghostClient.PaddingEnabled = &paddingEnabled
	ghostClient.JitterMS = cfg.JitterMS

	server := &socks5.Server{
		Addr: cfg.ListenAddress,
		Dial: ghostClient.Dial,
	}

	log.Printf(
		"Ghost client listening on %s -> %s",
		cfg.ListenAddress,
		cfg.ServerAddress,
	)

	if err := server.Start(); err != nil {
		log.Fatalf("SOCKS5 server stopped: %v", err)
	}
}
