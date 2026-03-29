package skills

// Skill represents a reusable prompt template that users can invoke on demand.
type Skill struct {
	Name        string
	Alias       string // short alias for quick invocation
	Description string
	Prompt      string // uses {input} as placeholder for user input
}

// Builtin returns all built-in skills.
func Builtin() []Skill {
	return []Skill{
		Translate(),
		Summarize(),
		Explain(),
		CodeReview(),
		Rewrite(),
		BulletPoints(),
	}
}

// Find returns a skill by name or alias (case-insensitive match).
func Find(name string) *Skill {
	for _, s := range Builtin() {
		if s.Name == name || s.Alias == name {
			return &s
		}
	}
	return nil
}

func Translate() Skill {
	return Skill{
		Name:        "translate",
		Alias:       "tr",
		Description: "Translate text between languages",
		Prompt:      "Translate the following text. If it's in Chinese, translate to English. If it's in English, translate to Chinese. Preserve formatting and tone.\n\nText:\n{input}",
	}
}

func Summarize() Skill {
	return Skill{
		Name:        "summarize",
		Alias:       "sum",
		Description: "Summarize text into key points",
		Prompt:      "Summarize the following text into concise bullet points. Capture all key information.\n\nText:\n{input}",
	}
}

func Explain() Skill {
	return Skill{
		Name:        "explain",
		Alias:       "eli5",
		Description: "Explain a concept simply",
		Prompt:      "Explain the following concept in simple, easy-to-understand terms. Use analogies where helpful.\n\nTopic:\n{input}",
	}
}

func CodeReview() Skill {
	return Skill{
		Name:        "codereview",
		Alias:       "cr",
		Description: "Review code for issues and improvements",
		Prompt:      "Review the following code. Identify bugs, security issues, performance problems, and suggest improvements. Be specific and actionable.\n\nCode:\n{input}",
	}
}

func Rewrite() Skill {
	return Skill{
		Name:        "rewrite",
		Alias:       "rw",
		Description: "Rewrite text to be clearer and more polished",
		Prompt:      "Rewrite the following text to be clearer, more concise, and more professional. Preserve the original meaning.\n\nText:\n{input}",
	}
}

func BulletPoints() Skill {
	return Skill{
		Name:        "bullets",
		Alias:       "bp",
		Description: "Convert text into organized bullet points",
		Prompt:      "Convert the following text into well-organized bullet points with clear hierarchy.\n\nText:\n{input}",
	}
}
