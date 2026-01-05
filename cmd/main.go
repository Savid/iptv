// Package main is the entry point for the IPTV proxy.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/savid/iptv/internal/config"
	"github.com/savid/iptv/internal/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	cfg = config.DefaultConfig()
	log = logrus.New()
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "iptv",
		Short: "IPTV proxy with HDHomeRun emulation for Plex",
		Long:  `A simple IPTV proxy that emulates an HDHomeRun tuner for Plex Live TV.`,
		RunE:  run,
	}

	// Required flags
	rootCmd.Flags().StringVar(&cfg.M3UURL, "m3u", "", "M3U playlist URL (required)")
	rootCmd.Flags().StringVar(&cfg.EPGURL, "epg", "", "EPG XML URL (required)")
	rootCmd.Flags().StringVar(&cfg.BaseURL, "base", "", "Base URL for stream URLs (required)")

	if err := rootCmd.MarkFlagRequired("m3u"); err != nil {
		log.WithError(err).Fatal("Failed to mark m3u flag as required")
	}

	if err := rootCmd.MarkFlagRequired("epg"); err != nil {
		log.WithError(err).Fatal("Failed to mark epg flag as required")
	}

	if err := rootCmd.MarkFlagRequired("base"); err != nil {
		log.WithError(err).Fatal("Failed to mark base flag as required")
	}

	// Server flags
	rootCmd.Flags().StringVar(&cfg.BindAddr, "bind", cfg.BindAddr, "Bind address")
	rootCmd.Flags().IntVar(&cfg.Port, "port", cfg.Port, "Port number")
	rootCmd.Flags().StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")

	// HDHomeRun flags
	rootCmd.Flags().IntVar(&cfg.TunerCount, "tuner-count", cfg.TunerCount, "Number of tuners to advertise")
	rootCmd.Flags().StringVar(&cfg.DeviceID, "device-id", cfg.DeviceID, "Device ID")
	rootCmd.Flags().StringVar(&cfg.DeviceName, "device-name", cfg.DeviceName, "Device name prefix shown in Plex")

	// Data flags
	rootCmd.Flags().DurationVar(&cfg.RefreshInterval, "refresh", cfg.RefreshInterval, "Data refresh interval")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Configure logger
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return err
	}

	log.SetLevel(level)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})

	// Validate config
	if err := cfg.Validate(); err != nil {
		return err
	}

	log.WithFields(logrus.Fields{
		"m3u":  cfg.M3UURL,
		"epg":  cfg.EPGURL,
		"base": cfg.BaseURL,
	}).Info("Starting IPTV proxy")

	// Create and start server
	srv := server.NewServer(log, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		return err
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("Received shutdown signal")

	return srv.Stop()
}
