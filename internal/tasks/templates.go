package tasks

// Built-in task templates that can be added via Telegram commands.
// Each template defines a name, prompt, and suggested cron schedule.

// Template represents a built-in scheduled task template.
type Template struct {
	Name        string
	Description string
	Prompt      string
	CronExpr    string // Default cron expression (6-field with seconds)
}

// Builtin returns all built-in task templates.
func Builtin() []Template {
	return []Template{
		DailyBriefing(),
		StockAnalysis(),
		WeeklyFinance(),
		NewsDigest(),
		EnglishLearning(),
		DailyMemoryExtract(),
	}
}

// DailyBriefing generates a personalized morning briefing.
func DailyBriefing() Template {
	return Template{
		Name:        "daily-briefing",
		Description: "Personalized morning briefing with weather, calendar, and priorities",
		Prompt: `Generate a concise morning briefing for today. Include:
1. Today's date and day of the week
2. A motivational quote to start the day
3. Key things to focus on based on my goals and recent context
4. Any relevant reminders or follow-ups from recent conversations

Keep it concise, friendly, and actionable. Use bullet points.`,
		CronExpr: "0 0 8 * * *", // 8:00 AM daily
	}
}

// StockAnalysis generates a stock portfolio analysis.
func StockAnalysis() Template {
	return Template{
		Name:        "stock-analysis",
		Description: "Daily stock portfolio analysis and market overview",
		Prompt: `Provide a brief stock market and portfolio analysis:
1. Major market indices summary (S&P 500, NASDAQ, etc.)
2. Key market movers and notable news
3. Analysis of my holdings if you know them from memory
4. Any actionable insights or alerts

Be factual, concise, and note if any data may be outdated. Format with bullet points and keep under 500 words.`,
		CronExpr: "0 30 9 * * 1-5", // 9:30 AM weekdays
	}
}

// WeeklyFinance generates a weekly financial report.
func WeeklyFinance() Template {
	return Template{
		Name:        "weekly-finance",
		Description: "Weekly financial review and planning report",
		Prompt: `Generate a weekly financial review:
1. Summary of this week's market performance
2. Portfolio performance summary (if holdings are known)
3. Key economic events coming next week
4. Spending/saving insights if any context is available
5. Financial action items or reminders

Keep it structured and actionable. Use clear sections with headers.`,
		CronExpr: "0 0 10 * * 0", // 10:00 AM Sunday
	}
}

// NewsDigest generates a curated news digest.
func NewsDigest() Template {
	return Template{
		Name:        "news-digest",
		Description: "Curated daily news digest across tech, finance, and world events",
		Prompt: `Compile a brief daily news digest covering:
1. Top 3 tech/AI news stories
2. Top 3 financial/market news stories
3. Top 2 world news stories
4. One interesting or fun story

For each item, provide a one-sentence summary and why it matters. Keep the entire digest under 400 words.`,
		CronExpr: "0 0 7 * * *", // 7:00 AM daily
	}
}

// EnglishLearning generates daily English learning content.
func EnglishLearning() Template {
	return Template{
		Name:        "english-learning",
		Description: "Daily English learning with vocabulary, idioms, and practice",
		Prompt: `Create a daily English learning session:
1. Word of the Day: An advanced vocabulary word with definition, pronunciation hint, etymology, and 2 example sentences
2. Idiom of the Day: A common English idiom with meaning and usage example
3. Grammar Tip: One useful grammar point with correct/incorrect examples
4. Mini Exercise: A short fill-in-the-blank or sentence construction exercise (provide the answer at the end)

Make it engaging and practical for a Chinese-speaking English learner at intermediate-advanced level.`,
		CronExpr: "0 30 7 * * *", // 7:30 AM daily
	}
}

// DailyMemoryExtract triggers memory extraction from the day's conversations.
func DailyMemoryExtract() Template {
	return Template{
		Name:        "daily-memory-extract",
		Description: "Extract and consolidate memories from today's conversations",
		Prompt: `Review today's conversation history and:
1. Identify any new facts, preferences, or goals mentioned
2. Note any decisions made or action items agreed upon
3. Flag any follow-ups needed
4. Summarize key topics discussed today

Output a brief summary of what was learned and remembered today.`,
		CronExpr: "0 0 23 * * *", // 11:00 PM daily
	}
}
