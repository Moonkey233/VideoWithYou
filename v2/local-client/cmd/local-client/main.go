package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"videowithyou/v2/local-client/internal/client"
	"videowithyou/v2/local-client/internal/config"
	"videowithyou/v2/local-client/internal/extws"
)

func main() {
	configPath := flag.String("config", filepath.Join("local-client", "config.json"), "config path")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host := extws.NewHost(cfg.ExtListenAddr, cfg.ExtListenPath, log.Default())
	c := client.New(cfg, *configPath, host, log.Default())
	c.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	cancel()
}
