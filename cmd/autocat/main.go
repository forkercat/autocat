package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wjunhao/autocat/internal/config"
	"github.com/wjunhao/autocat/internal/db"
	"github.com/wjunhao/autocat/internal/memory"
	"github.com/wjunhao/autocat/internal/scheduler"
	"github.com/wjunhao/autocat/internal/session"
	"github.com/wjunhao/autocat/internal/telegram"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("[INFO] AutoCat starting...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Configuration error: %v", err)
	}
	log.Printf("[INFO] Config loaded: model=%s, timezone=%s", cfg.ClaudeModel, cfg.Timezone)

	// Initialize database
	database, err := db.Init(cfg.DataDir)
	if err != nil {
		log.Fatalf("[FATAL] Database error: %v", err)
	}
	defer database.Close()
	log.Printf("[INFO] Database initialized")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create scheduler first (bot needs it for task management)
	sched := scheduler.New(database, cfg, nil)

	// Create Telegram bot (pass scheduler; bot's SendMessage wired into scheduler below)
	bot, err := telegram.New(cfg, database, sched)
	if err != nil {
		log.Fatalf("[FATAL] Telegram bot error: %v", err)
	}

	// Wire bot's send function into scheduler
	sched.SetSender(bot.SendMessage)

	// Start scheduler
	if err := sched.Start(); err != nil {
		log.Fatalf("[FATAL] Scheduler error: %v", err)
	}
	defer sched.Stop()

	// Start daily reset routine
	go dailyReset(ctx, cfg, database)

	// Start memory cleanup routine
	go memoryCleanup(ctx, database)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[INFO] Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Start Telegram bot (blocking)
	bot.Start(ctx)

	log.Printf("[INFO] AutoCat stopped")
}

// dailyReset ends all active sessions at the configured hour.
func dailyReset(ctx context.Context, cfg *config.Config, database *sql.DB) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}

	for {
		now := time.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day(), cfg.DailyResetHour, 0, 0, 0, loc)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}

		sleepDuration := time.Until(next)
		log.Printf("[INFO] Next daily reset at %s (in %s)", next.Format(time.RFC3339), sleepDuration.Round(time.Minute))

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepDuration):
			count, err := session.EndAllActive(database)
			if err != nil {
				log.Printf("[ERROR] Daily reset failed: %v", err)
			} else {
				log.Printf("[INFO] Daily reset: ended %d active sessions", count)
			}
		}
	}
}

// memoryCleanup periodically removes expired memories.
func memoryCleanup(ctx context.Context, database *sql.DB) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := memory.CleanExpired(database)
			if err != nil {
				log.Printf("[ERROR] Memory cleanup failed: %v", err)
			} else if count > 0 {
				log.Printf("[INFO] Cleaned %d expired memories", count)
			}
		}
	}
}
