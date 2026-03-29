package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/wjunhao/autocat/internal/claude"
	"github.com/wjunhao/autocat/internal/config"
	"github.com/wjunhao/autocat/internal/memory"
)

// MessageSender is a function that sends a message to a chat.
type MessageSender func(chatID string, text string) error

// Task represents a scheduled task stored in the database.
type Task struct {
	ID             int64
	Name           string
	ChatID         string
	Prompt         string
	CronExpression string
	Enabled        bool
	LastRun        sql.NullInt64
	NextRun        sql.NullInt64
	CreatedAt      int64
	UpdatedAt      int64
}

// Scheduler manages cron-based scheduled tasks.
type Scheduler struct {
	cron   *cron.Cron
	db     *sql.DB
	cfg    *config.Config
	send   MessageSender
	mu     sync.Mutex
	jobs   map[int64]cron.EntryID // taskID -> cron entryID
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Scheduler. send may be nil and set later via SetSender.
func New(db *sql.DB, cfg *config.Config, send MessageSender) *Scheduler {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Printf("[WARN] Invalid timezone %s, using UTC: %v", cfg.Timezone, err)
		loc = time.UTC
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		cron:   cron.New(cron.WithLocation(loc), cron.WithSeconds()),
		db:     db,
		cfg:    cfg,
		send:   send,
		jobs:   make(map[int64]cron.EntryID),
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetSender wires in the message sender after construction (breaks init cycle).
func (s *Scheduler) SetSender(send MessageSender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.send = send
}

// Start loads all enabled tasks and starts the cron scheduler.
func (s *Scheduler) Start() error {
	tasks, err := s.ListEnabled()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	for _, task := range tasks {
		if err := s.scheduleTask(task); err != nil {
			log.Printf("[WARN] Failed to schedule task %q: %v", task.Name, err)
		}
	}

	s.cron.Start()
	log.Printf("[INFO] Scheduler started with %d tasks", len(tasks))
	return nil
}

// Stop gracefully stops the scheduler and waits for running tasks to finish.
func (s *Scheduler) Stop() {
	s.cancel() // signal running tasks to stop
	cronCtx := s.cron.Stop()
	<-cronCtx.Done()
	s.wg.Wait() // wait for in-flight task executions
	log.Printf("[INFO] Scheduler stopped")
}

// AddTask creates and schedules a new task.
func (s *Scheduler) AddTask(name, chatID, prompt, cronExpr string) (*Task, error) {
	// Validate cron expression
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(cronExpr); err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	now := time.Now().UnixMilli()
	result, err := s.db.Exec(
		"INSERT INTO scheduled_tasks (name, chat_id, prompt, cron_expression, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)",
		name, chatID, prompt, cronExpr, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	id, _ := result.LastInsertId()
	task := &Task{
		ID:             id,
		Name:           name,
		ChatID:         chatID,
		Prompt:         prompt,
		CronExpression: cronExpr,
		Enabled:        true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.scheduleTask(*task); err != nil {
		return nil, fmt.Errorf("schedule task: %w", err)
	}

	log.Printf("[INFO] Task added: %s (cron: %s)", name, cronExpr)
	return task, nil
}

// RemoveTask disables and unschedules a task.
func (s *Scheduler) RemoveTask(taskID int64) error {
	s.mu.Lock()
	if entryID, ok := s.jobs[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, taskID)
	}
	s.mu.Unlock()

	_, err := s.db.Exec(
		"UPDATE scheduled_tasks SET enabled = 0, updated_at = ? WHERE id = ?",
		time.Now().UnixMilli(), taskID,
	)
	return err
}

// ListAll returns all scheduled tasks.
func (s *Scheduler) ListAll() ([]Task, error) {
	return s.queryTasks("SELECT id, name, chat_id, prompt, cron_expression, enabled, last_run, next_run, created_at, updated_at FROM scheduled_tasks ORDER BY name")
}

// ListEnabled returns all enabled tasks.
func (s *Scheduler) ListEnabled() ([]Task, error) {
	return s.queryTasks("SELECT id, name, chat_id, prompt, cron_expression, enabled, last_run, next_run, created_at, updated_at FROM scheduled_tasks WHERE enabled = 1 ORDER BY name")
}

func (s *Scheduler) scheduleTask(task Task) error {
	entryID, err := s.cron.AddFunc(task.CronExpression, func() {
		s.executeTask(task)
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.jobs[task.ID] = entryID
	s.mu.Unlock()
	return nil
}

func (s *Scheduler) executeTask(task Task) {
	s.wg.Add(1)
	defer s.wg.Done()

	log.Printf("[INFO] Executing scheduled task: %s", task.Name)
	startedAt := time.Now().UnixMilli()

	// Record the run
	runResult, err := s.db.Exec(
		"INSERT INTO task_runs (task_id, started_at, status) VALUES (?, ?, 'running')",
		task.ID, startedAt,
	)
	if err != nil {
		log.Printf("[ERROR] Failed to record task run: %v", err)
		return
	}
	runID, _ := runResult.LastInsertId()

	// Build context with memories
	memCtx := memory.FormatForContext(s.db, task.ChatID, 20)
	systemPrompt := fmt.Sprintf("You are %s, a personal AI assistant. Today is %s.\n\n%s",
		s.cfg.AssistantName,
		time.Now().Format("2006-01-02 (Monday)"),
		memCtx,
	)

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	resp, err := claude.Invoke(ctx, s.cfg, claude.InvokeOptions{
		Prompt:       task.Prompt,
		SystemPrompt: systemPrompt,
		MaxTurns:     3,
		Timeout:      5 * time.Minute,
	})

	finishedAt := time.Now().UnixMilli()

	if err != nil || (resp != nil && resp.Error != "") {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = resp.Error
		}
		log.Printf("[ERROR] Task %s failed: %s", task.Name, errMsg)

		if _, dbErr := s.db.Exec(
			"UPDATE task_runs SET finished_at = ?, status = 'error', error = ? WHERE id = ?",
			finishedAt, errMsg, runID,
		); dbErr != nil {
			log.Printf("[ERROR] Failed to update task_run %d to error: %v", runID, dbErr)
		}
		return
	}

	// Send result to chat
	if err := s.send(task.ChatID, resp.Text); err != nil {
		log.Printf("[ERROR] Failed to send task result: %v", err)
	}

	// Update run record
	if _, err := s.db.Exec(
		"UPDATE task_runs SET finished_at = ?, status = 'success', result = ? WHERE id = ?",
		finishedAt, truncate(resp.Text, 1000), runID,
	); err != nil {
		log.Printf("[ERROR] Failed to update task_run %d to success: %v", runID, err)
	}

	// Update task last_run
	if _, err := s.db.Exec(
		"UPDATE scheduled_tasks SET last_run = ?, updated_at = ? WHERE id = ?",
		finishedAt, finishedAt, task.ID,
	); err != nil {
		log.Printf("[ERROR] Failed to update task %d last_run: %v", task.ID, err)
	}

	log.Printf("[INFO] Task %s completed in %dms", task.Name, finishedAt-startedAt)
}

func (s *Scheduler) queryTasks(query string, args ...any) ([]Task, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var enabled int
		if err := rows.Scan(&t.ID, &t.Name, &t.ChatID, &t.Prompt, &t.CronExpression, &enabled, &t.LastRun, &t.NextRun, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Enabled = enabled == 1
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
