package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"

func main() {
	// Subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Printf("kfsSms %s\n", version)
			return
		case "help":
			fmt.Println("Usage: kfsSms [config.yml]")
			fmt.Println("       kfsSms version")
			return
		}
	}

	// Load config
	configPath := ""
	if len(os.Args) > 1 && os.Args[1] != "serve" {
		configPath = os.Args[1]
	} else if len(os.Args) > 2 {
		configPath = os.Args[2]
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	slog.Info("kfsSms starting", "version", version, "port", cfg.Sms.PortName)

	// Check for updates
	if cfg.Update.Owner != "" && cfg.Update.Repo != "" {
		newPath, err := CheckUpdate(cfg.Update.Owner, cfg.Update.Repo, version)
		if err != nil {
			slog.Warn("update check failed", "err", err)
		}
		if newPath != "" {
			slog.Info("update available, exiting for swap", "path", newPath)
			os.Exit(updateExitCode)
		}
	}

	// Open modem
	var modem *Modem
	if cfg.Sms.OpenModem {
		modem = NewModem(cfg.Sms.PortName, cfg.Sms.BaudRate)
		if err := modem.Open(cfg.Msisdn.Pin); err != nil {
			slog.Error("modem open failed", "err", err)
			os.Exit(1)
		}
		defer modem.Close()
		slog.Info("modem ready", "port", cfg.Sms.PortName)
	} else {
		slog.Warn("modem disabled (openModem=false)")
		modem = &Modem{} // dummy — will fail on actual operations
	}

	// Create REST client
	client := NewSmsClient(cfg.Api.BaseUrl, cfg.Api.User, cfg.Api.Password)

	// Run with graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	defer cancel()

	Run(ctx, cfg, modem, client)
	slog.Info("kfsSms stopped")
}
