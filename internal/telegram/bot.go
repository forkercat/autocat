package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/wjunhao/autocat/internal/claude"
	"github.com/wjunhao/autocat/internal/config"
	"github.com/wjunhao/autocat/internal/memory"
	"github.com/wjunhao/autocat/internal/scheduler"
	"github.com/wjunhao/autocat/internal/security"
	"github.com/wjunhao/autocat/internal/session"
	"github.com/wjunhao/autocat/internal/tasks"
)

// Bot wraps a Telegram bot with Claude integration.
type Bot struct {
	api         *tgbotapi.BotAPI
	cfg         *config.Config
	db          *sql.DB
	scheduler   *scheduler.Scheduler
	rateLimiter *security.RateLimiter
	mu          sync.Mutex
	activeLocks map[string]bool // chatID -> processing
	wg          sync.WaitGroup
}

// New creates a new Telegram bot.
func New(cfg *config.Config, db *sql.DB, sched *scheduler.Scheduler) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	api.Debug = cfg.Debug
	log.Printf("[INFO] Telegram bot authorized as @%s", api.Self.UserName)

	return &Bot{
		api:         api,
		cfg:         cfg,
		db:          db,
		scheduler:   sched,
		rateLimiter: security.NewRateLimiter(),
		activeLocks: make(map[string]bool),
	}, nil
}

// SendMessage sends a text message to a chat (implements scheduler.MessageSender).
func (b *Bot) SendMessage(chatID string, text string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	// Telegram has a 4096 char limit per message
	chunks := splitMessage(text, 4000)
	for _, chunk := range chunks {
		msg := tgbotapi.NewMessage(id, chunk)
		msg.ParseMode = "Markdown"
		if _, err := b.api.Send(msg); err != nil {
			// Retry without Markdown if parsing fails
			msg.ParseMode = ""
			if _, err := b.api.Send(msg); err != nil {
				return fmt.Errorf("send message: %w", err)
			}
		}
	}
	return nil
}

// Start begins listening for Telegram updates.
// Blocks until ctx is cancelled. Waits for in-flight handlers before returning.
func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	log.Printf("[INFO] Telegram bot started, listening for messages...")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] Telegram bot stopping, waiting for in-flight handlers...")
			b.api.StopReceivingUpdates()
			b.rateLimiter.Stop()
			b.wg.Wait()
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			b.wg.Add(1)
			go func(msg *tgbotapi.Message) {
				defer b.wg.Done()
				b.handleMessage(ctx, msg)
			}(update.Message)
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := strconv.FormatInt(msg.Chat.ID, 10)

	// Security: check allowlist
	if !b.cfg.IsUserAllowed(userID) {
		log.Printf("[WARN] Unauthorized message from user %d", userID)
		return
	}

	// Rate limiting
	userIDStr := strconv.FormatInt(userID, 10)
	if !b.rateLimiter.Allow(userIDStr) {
		b.replyText(msg.Chat.ID, "Rate limit exceeded. Please wait a moment.")
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(text, "/") {
		b.handleCommand(ctx, msg, chatID, text)
		return
	}

	// Handle regular messages
	b.handleChat(ctx, msg, chatID, text)
}

func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message, chatID, text string) {
	parts := strings.Fields(text)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/start":
		b.replyText(msg.Chat.ID, fmt.Sprintf(
			"Hi! I'm %s, your personal AI assistant.\n\nCommands:\n"+
				"/new - Start a new session\n"+
				"/tasks - List scheduled tasks\n"+
				"/addtask - Add a task from templates\n"+
				"/memory - View recent memories\n"+
				"/status - Show bot status\n"+
				"/help - Show this help",
			b.cfg.AssistantName,
		))

	case "/new":
		_, err := session.Create(b.db, chatID)
		if err != nil {
			b.replyText(msg.Chat.ID, "Failed to create new session.")
			return
		}
		b.replyText(msg.Chat.ID, "New session started! Previous context has been saved.")

	case "/tasks":
		taskList, err := b.scheduler.ListAll()
		if err != nil {
			b.replyText(msg.Chat.ID, "Failed to list tasks.")
			return
		}
		if len(taskList) == 0 {
			b.replyText(msg.Chat.ID, "No scheduled tasks. Use /addtask to add one.")
			return
		}
		var sb strings.Builder
		sb.WriteString("Scheduled tasks:\n\n")
		for _, t := range taskList {
			status := "enabled"
			if !t.Enabled {
				status = "disabled"
			}
			sb.WriteString(fmt.Sprintf("- %s [%s] (cron: %s)\n", t.Name, status, t.CronExpression))
		}
		b.replyText(msg.Chat.ID, sb.String())

	case "/addtask":
		templates := tasks.Builtin()
		var sb strings.Builder
		sb.WriteString("Available task templates:\n\n")
		for i, t := range templates {
			sb.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, t.Name, t.Description))
		}
		sb.WriteString("\nReply with the template number to add it, e.g.: /enable 1")
		b.replyText(msg.Chat.ID, sb.String())

	case "/enable":
		if len(parts) < 2 {
			b.replyText(msg.Chat.ID, "Usage: /enable <number>")
			return
		}
		idx, err := strconv.Atoi(parts[1])
		if err != nil || idx < 1 || idx > len(tasks.Builtin()) {
			b.replyText(msg.Chat.ID, "Invalid template number.")
			return
		}
		tmpl := tasks.Builtin()[idx-1]
		task, err := b.scheduler.AddTask(tmpl.Name, chatID, tmpl.Prompt, tmpl.CronExpr)
		if err != nil {
			b.replyText(msg.Chat.ID, fmt.Sprintf("Failed to add task: %v", err))
			return
		}
		b.replyText(msg.Chat.ID, fmt.Sprintf("Task '%s' added! (cron: %s)\nTask ID: %d", task.Name, task.CronExpression, task.ID))

	case "/disable":
		if len(parts) < 2 {
			b.replyText(msg.Chat.ID, "Usage: /disable <task_id>")
			return
		}
		taskID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			b.replyText(msg.Chat.ID, "Invalid task ID.")
			return
		}
		if err := b.scheduler.RemoveTask(taskID); err != nil {
			b.replyText(msg.Chat.ID, fmt.Sprintf("Failed to disable task: %v", err))
			return
		}
		b.replyText(msg.Chat.ID, "Task disabled.")

	case "/memory":
		entries, err := memory.Query(b.db, chatID, "", 10)
		if err != nil {
			b.replyText(msg.Chat.ID, "Failed to query memories.")
			return
		}
		if len(entries) == 0 {
			b.replyText(msg.Chat.ID, "No memories stored yet.")
			return
		}
		var sb strings.Builder
		sb.WriteString("Recent memories:\n\n")
		for _, e := range entries {
			t := time.UnixMilli(e.CreatedAt).Format("01/02")
			sb.WriteString(fmt.Sprintf("- [%s] [%s] %s\n", t, e.Category, e.Content))
		}
		b.replyText(msg.Chat.ID, sb.String())

	case "/status":
		sessID := "(none)"
		if sess, err := session.GetActive(b.db, chatID); err == nil && sess != nil {
			sessID = sess.ID
		}
		taskCount := 0
		if taskList, err := b.scheduler.ListEnabled(); err == nil {
			taskCount = len(taskList)
		}
		b.replyText(msg.Chat.ID, fmt.Sprintf(
			"Status:\n- Model: %s\n- Session: %s\n- Active tasks: %d\n- Timezone: %s",
			b.cfg.ClaudeModel, sessID, taskCount, b.cfg.Timezone,
		))

	case "/help":
		b.replyText(msg.Chat.ID, fmt.Sprintf(
			"%s Commands:\n\n"+
				"/start - Welcome message\n"+
				"/new - Start a new session\n"+
				"/tasks - List scheduled tasks\n"+
				"/addtask - Show task templates\n"+
				"/enable <n> - Enable a task template\n"+
				"/disable <id> - Disable a task\n"+
				"/memory - View recent memories\n"+
				"/status - Show bot status\n"+
				"/help - Show this help\n\n"+
				"Or just send me a message to chat!",
			b.cfg.AssistantName,
		))

	default:
		b.replyText(msg.Chat.ID, "Unknown command. Use /help to see available commands.")
	}
}

func (b *Bot) handleChat(ctx context.Context, msg *tgbotapi.Message, chatID, text string) {
	// Prevent concurrent processing for same chat
	b.mu.Lock()
	if b.activeLocks[chatID] {
		b.mu.Unlock()
		b.replyText(msg.Chat.ID, "Processing previous message, please wait...")
		return
	}
	b.activeLocks[chatID] = true
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.activeLocks, chatID)
		b.mu.Unlock()
	}()

	// Send typing indicator
	typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
	b.api.Send(typing)

	// Sanitize input
	text = security.SanitizeInput(text)

	// Get or create session
	sess, err := session.GetActive(b.db, chatID)
	if err != nil {
		log.Printf("[ERROR] Failed to get session: %v", err)
		b.replyText(msg.Chat.ID, "Internal error. Please try again.")
		return
	}

	// Store user message
	b.storeMessage(chatID, strconv.FormatInt(msg.From.ID, 10), msg.From.FirstName, text, "user", sess.ID)

	// Build system prompt with memory context
	memCtx := memory.FormatForContext(b.db, chatID, 20)
	systemPrompt := fmt.Sprintf(
		"You are %s, a helpful personal AI assistant. Today is %s. Be concise, friendly, and helpful. Respond in the same language the user uses.\n\n%s",
		b.cfg.AssistantName,
		time.Now().Format("2006-01-02 (Monday)"),
		memCtx,
	)

	// Invoke Claude
	opts := claude.DefaultMultiTurn(text, systemPrompt, "")
	if sess.ClaudeSessionID.Valid {
		opts.SessionID = sess.ClaudeSessionID.String
	}

	resp, err := claude.Invoke(ctx, b.cfg, opts)
	if err != nil {
		log.Printf("[ERROR] Claude invocation failed: %v", err)
		b.replyText(msg.Chat.ID, "Sorry, I encountered an error. Please try again.")
		return
	}
	if resp.Error != "" {
		log.Printf("[ERROR] Claude error: %s", resp.Error)
		b.replyText(msg.Chat.ID, "Sorry, I encountered an error processing your request.")
		return
	}

	// Update Claude session ID if new
	if resp.SessionID != "" && (!sess.ClaudeSessionID.Valid || sess.ClaudeSessionID.String != resp.SessionID) {
		session.UpdateClaudeSessionID(b.db, sess.ID, resp.SessionID)
	}

	// Store assistant response
	b.storeMessage(chatID, "assistant", b.cfg.AssistantName, resp.Text, "assistant", sess.ID)

	// Send response
	b.SendMessage(chatID, resp.Text)
}

func (b *Bot) storeMessage(chatID, sender, senderName, content, role, sessionID string) {
	_, err := b.db.Exec(
		"INSERT INTO messages (chat_id, sender, sender_name, content, role, timestamp, session_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		chatID, sender, senderName, content, role, time.Now().UnixMilli(), sessionID,
	)
	if err != nil {
		log.Printf("[ERROR] Failed to store message: %v", err)
	}
}

func (b *Bot) replyText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("[ERROR] Failed to send reply: %v", err)
	}
}

func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		// Try to split at a newline within the chunk
		idx := strings.LastIndex(text[:maxLen], "\n")
		if idx <= 0 {
			// No good split point — hard cut at maxLen
			idx = maxLen
		}
		chunks = append(chunks, text[:idx])
		text = strings.TrimLeft(text[idx:], "\n")
	}
	return chunks
}
