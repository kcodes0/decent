package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kcodes0/decent/internal/node"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	configPath := flag.String("config", "decent-node.toml", "path to the node config file")
	flag.Parse()

	cfg, err := node.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	daemon := node.NewDaemon(cfg)
	if err := daemon.Run(ctx); err != nil {
		log.Fatalf("decent-node: %v", err)
	}
}
