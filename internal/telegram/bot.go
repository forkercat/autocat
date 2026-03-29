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

	"github.com/forkercat/autocat/internal/claude"
	"github.com/forkercat/autocat/internal/config"
	"github.com/forkercat/autocat/internal/gws"
	"github.com/forkercat/autocat/internal/memory"
	"github.com/forkercat/autocat/internal/metrics"
	"github.com/forkercat/autocat/internal/personalize"
	"github.com/forkercat/autocat/internal/scheduler"
	"github.com/forkercat/autocat/internal/security"
	"github.com/forkercat/autocat/internal/session"
	"github.com/forkercat/autocat/internal/skills"
	"github.com/forkercat/autocat/internal/tasks"
)

// Bot wraps a Telegram bot with Claude integration.
type Bot struct {
	api         *tgbotapi.BotAPI
	cfg         *config.Config
	db          *sql.DB
	scheduler   *scheduler.Scheduler
	rateLimiter *security.RateLimiter
	mu             sync.Mutex
	activeLocks    map[string]bool   // chatID -> processing
	modelOverrides map[string]string // chatID -> model override
	wg             sync.WaitGroup
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
		rateLimiter:    security.NewRateLimiter(),
		activeLocks:    make(map[string]bool),
		modelOverrides: make(map[string]string),
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

	userIDStr := strconv.FormatInt(userID, 10)
	m := metrics.Get()
	m.MessagesReceived.Add(1)

	// Security: check allowlist
	if !b.cfg.IsUserAllowed(userID) {
		m.Unauthorized.Add(1)
		security.AuditLog(security.AuditUnauthorized, userIDStr, fmt.Sprintf("chatID=%s", chatID))
		return
	}

	// Rate limiting
	if !b.rateLimiter.Allow(userIDStr) {
		m.RateLimited.Add(1)
		security.AuditLog(security.AuditRateLimited, userIDStr, "")
		b.replyText(msg.Chat.ID, "Rate limit exceeded. Please wait a moment.")
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(text, "/") {
		security.AuditLog(security.AuditCommand, userIDStr, text)
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
		help := fmt.Sprintf(
			"Hi! I'm %s, your personal AI assistant.\n\nCommands:\n"+
				"/new - Start a new session\n"+
				"/model - Switch model (sonnet/opus)\n"+
				"/skill - Use a skill (translate, summarize...)\n"+
				"/instructions - Set custom instructions\n"+
				"/tasks - List scheduled tasks\n"+
				"/addtask - Add a task from templates\n"+
				"/memory - View recent memories\n"+
				"/status - Show bot status\n"+
				"/help - Show this help",
			b.cfg.AssistantName,
		)
		if b.cfg.GWSEnabled {
			help += "\n\nGoogle Workspace:\n/gmail - Inbox triage\n/calendar - Today's agenda\n/gtasks - Task list"
		}
		b.replyText(msg.Chat.ID, help)

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
			b.effectiveModel(chatID), sessID, taskCount, b.cfg.Timezone,
		))

	case "/gmail":
		if !b.cfg.GWSEnabled {
			b.replyText(msg.Chat.ID, "Google Workspace integration is not enabled. Set GWS_ENABLED=true in .env.")
			return
		}
		typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
		b.api.Send(typing)

		raw, err := gws.GmailTriage(ctx)
		if err != nil {
			b.replyText(msg.Chat.ID, fmt.Sprintf("Failed to fetch Gmail: %v", err))
			return
		}
		b.summarizeAndReply(ctx, msg, chatID, raw, "Summarize this Gmail inbox triage into a concise, readable format. Group by importance. Use the user's language.")

	case "/calendar", "/cal":
		if !b.cfg.GWSEnabled {
			b.replyText(msg.Chat.ID, "Google Workspace integration is not enabled. Set GWS_ENABLED=true in .env.")
			return
		}
		typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
		b.api.Send(typing)

		raw, err := gws.CalendarAgenda(ctx, b.cfg.Timezone)
		if err != nil {
			b.replyText(msg.Chat.ID, fmt.Sprintf("Failed to fetch calendar: %v", err))
			return
		}
		b.summarizeAndReply(ctx, msg, chatID, raw, "Summarize today's calendar events into a concise schedule. Highlight any conflicts or important meetings. Use the user's language.")

	case "/gtasks":
		if !b.cfg.GWSEnabled {
			b.replyText(msg.Chat.ID, "Google Workspace integration is not enabled. Set GWS_ENABLED=true in .env.")
			return
		}
		typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
		b.api.Send(typing)

		raw, err := gws.TasksList(ctx)
		if err != nil {
			b.replyText(msg.Chat.ID, fmt.Sprintf("Failed to fetch tasks: %v", err))
			return
		}
		b.summarizeAndReply(ctx, msg, chatID, raw, "Summarize these Google Tasks into a clear, organized to-do list. Use the user's language.")

	case "/instructions", "/inst":
		if len(parts) < 2 {
			current, err := personalize.GetInstructions(b.db, chatID)
			if err != nil {
				b.replyText(msg.Chat.ID, "Failed to load instructions.")
				return
			}
			if current == "" {
				b.replyText(msg.Chat.ID, "No custom instructions set.\n\nUsage:\n/instructions <text> — set instructions\n/instructions clear — remove instructions\n\nExample:\n/instructions Always respond in English. I'm a senior Go developer working on backend services.")
				return
			}
			b.replyText(msg.Chat.ID, fmt.Sprintf("Current instructions:\n\n%s\n\nUse /instructions clear to remove.", current))
			return
		}

		arg := strings.TrimSpace(text[len(parts[0]):])
		if strings.ToLower(arg) == "clear" {
			if err := personalize.ClearInstructions(b.db, chatID); err != nil {
				b.replyText(msg.Chat.ID, "Failed to clear instructions.")
				return
			}
			b.replyText(msg.Chat.ID, "Custom instructions cleared.")
			return
		}

		if err := personalize.SetInstructions(b.db, chatID, arg); err != nil {
			b.replyText(msg.Chat.ID, "Failed to save instructions.")
			return
		}
		b.replyText(msg.Chat.ID, "Instructions saved! They will be included in all future messages.")

	case "/skill":
		if len(parts) < 2 {
			available := skills.Builtin()
			var sb strings.Builder
			sb.WriteString("Available skills:\n\n")
			for _, s := range available {
				sb.WriteString(fmt.Sprintf("/%s (/%s) — %s\n", s.Name, s.Alias, s.Description))
			}
			sb.WriteString("\nUsage: /skill <name> <input>\nOr use the alias directly: /tr hello world")
			b.replyText(msg.Chat.ID, sb.String())
			return
		}

		skillName := parts[1]
		skill := skills.Find(skillName)
		if skill == nil {
			b.replyText(msg.Chat.ID, fmt.Sprintf("Unknown skill: %s. Use /skill to see available skills.", skillName))
			return
		}

		if len(parts) < 3 {
			b.replyText(msg.Chat.ID, fmt.Sprintf("Usage: /skill %s <input>", skill.Name))
			return
		}

		input := strings.TrimSpace(text[len(parts[0])+1+len(parts[1]):])
		b.invokeSkill(ctx, msg, chatID, skill, input)

	case "/model":
		b.mu.Lock()
		current := b.modelOverrides[chatID]
		b.mu.Unlock()
		if current == "" {
			current = b.cfg.ClaudeModel
		}

		if len(parts) < 2 {
			other := "claude-opus-4-6"
			if current == "claude-opus-4-6" {
				other = "claude-sonnet-4-6"
			}
			b.replyText(msg.Chat.ID, fmt.Sprintf("Current model: %s\n\nSwitch with: /model %s", current, other))
			return
		}

		target := parts[1]
		// Allow short aliases
		switch strings.ToLower(target) {
		case "sonnet", "claude-sonnet-4-6":
			target = "claude-sonnet-4-6"
		case "opus", "claude-opus-4-6":
			target = "claude-opus-4-6"
		default:
			b.replyText(msg.Chat.ID, "Usage: /model sonnet or /model opus")
			return
		}

		b.mu.Lock()
		b.modelOverrides[chatID] = target
		b.mu.Unlock()
		b.replyText(msg.Chat.ID, fmt.Sprintf("Model switched to %s", target))

	case "/help":
		help := fmt.Sprintf(
			"%s Commands:\n\n"+
				"/start - Welcome message\n"+
				"/new - Start a new session\n"+
				"/model - Switch model (sonnet/opus)\n"+
				"/skill - List available skills\n"+
				"/instructions - Set custom instructions\n"+
				"/tasks - List scheduled tasks\n"+
				"/addtask - Show task templates\n"+
				"/enable <n> - Enable a task template\n"+
				"/disable <id> - Disable a task\n"+
				"/memory - View recent memories\n"+
				"/status - Show bot status\n"+
				"/help - Show this help\n\n"+
				"Or just send me a message to chat!",
			b.cfg.AssistantName,
		)
		if b.cfg.GWSEnabled {
			help += "\n\nGoogle Workspace:\n/gmail - Inbox triage\n/calendar (/cal) - Today's agenda\n/gtasks - Task list"
		}
		b.replyText(msg.Chat.ID, help)

	default:
		// Check if the command matches a skill name or alias
		skillName := strings.TrimPrefix(cmd, "/")
		if skill := skills.Find(skillName); skill != nil {
			if len(parts) < 2 {
				b.replyText(msg.Chat.ID, fmt.Sprintf("Usage: /%s <input>\n\n%s", skillName, skill.Description))
				return
			}
			input := strings.TrimSpace(text[len(parts[0]):])
			b.invokeSkill(ctx, msg, chatID, skill, input)
			return
		}
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

	// Build system prompt with memory context and custom instructions
	memCtx := memory.FormatForContext(b.db, chatID, 20)
	customInst, _ := personalize.GetInstructions(b.db, chatID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"You are %s, a helpful personal AI assistant. Today is %s. Be concise, friendly, and helpful. Respond in the same language the user uses.",
		b.cfg.AssistantName,
		time.Now().Format("2006-01-02 (Monday)"),
	))
	if customInst != "" {
		sb.WriteString("\n\n## Custom instructions\n\n")
		sb.WriteString(customInst)
	}
	if memCtx != "" {
		sb.WriteString("\n\n")
		sb.WriteString(memCtx)
	}
	systemPrompt := sb.String()

	// Invoke Claude
	m := metrics.Get()
	m.ClaudeInvocations.Add(1)

	opts := claude.DefaultMultiTurn(text, systemPrompt, "")
	opts.Model = b.effectiveModel(chatID)
	if sess.ClaudeSessionID.Valid {
		opts.SessionID = sess.ClaudeSessionID.String
	}

	resp, err := claude.Invoke(ctx, b.cfg, opts)
	if err != nil {
		m.ClaudeErrors.Add(1)
		log.Printf("[ERROR] Claude invocation failed: %v", err)
		b.replyText(msg.Chat.ID, "Sorry, I encountered an error. Please try again.")
		return
	}
	if resp.Error != "" {
		m.ClaudeErrors.Add(1)
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
	m.MessagesSent.Add(1)
	b.SendMessage(chatID, resp.Text)
}

func (b *Bot) summarizeAndReply(ctx context.Context, msg *tgbotapi.Message, chatID, rawData, instruction string) {
	if strings.TrimSpace(rawData) == "" {
		b.replyText(msg.Chat.ID, "No data returned.")
		return
	}

	prompt := fmt.Sprintf("%s\n\nRaw data:\n---\n%s\n---", instruction, rawData)

	m := metrics.Get()
	m.ClaudeInvocations.Add(1)

	opts := claude.DefaultSingleTurn(prompt, "")
	opts.Model = b.effectiveModel(chatID)

	resp, err := claude.Invoke(ctx, b.cfg, opts)
	if err != nil || resp.Error != "" {
		m.ClaudeErrors.Add(1)
		// Fallback: send raw data truncated
		b.SendMessage(chatID, truncateText(rawData, 3000))
		return
	}

	m.MessagesSent.Add(1)
	b.SendMessage(chatID, resp.Text)
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n\n...(truncated)"
}

func (b *Bot) invokeSkill(ctx context.Context, msg *tgbotapi.Message, chatID string, skill *skills.Skill, input string) {
	// Send typing indicator
	typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
	b.api.Send(typing)

	prompt := strings.Replace(skill.Prompt, "{input}", input, 1)

	m := metrics.Get()
	m.ClaudeInvocations.Add(1)

	opts := claude.DefaultSingleTurn(prompt, "")
	opts.Model = b.effectiveModel(chatID)

	resp, err := claude.Invoke(ctx, b.cfg, opts)
	if err != nil {
		m.ClaudeErrors.Add(1)
		log.Printf("[ERROR] Skill %s invocation failed: %v", skill.Name, err)
		b.replyText(msg.Chat.ID, "Sorry, skill execution failed. Please try again.")
		return
	}
	if resp.Error != "" {
		m.ClaudeErrors.Add(1)
		log.Printf("[ERROR] Skill %s error: %s", skill.Name, resp.Error)
		b.replyText(msg.Chat.ID, "Sorry, skill execution encountered an error.")
		return
	}

	m.MessagesSent.Add(1)
	b.SendMessage(chatID, resp.Text)
}

func (b *Bot) effectiveModel(chatID string) string {
	b.mu.Lock()
	m := b.modelOverrides[chatID]
	b.mu.Unlock()
	if m != "" {
		return m
	}
	return b.cfg.ClaudeModel
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
