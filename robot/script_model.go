package robot

import "fmt"

const (
	Tsend    TypeScript = "send"
	Trespond TypeScript = "respond"
	Tdirect  TypeScript = "direct"
)

type ScriptMiddleware func(script Script, next EnvelopHandler) EnvelopHandler

type CommandMiddleware func(command SlashCommand, next CommandHandler) CommandHandler

type Middleware interface {
	ScriptMiddleware(script Script, next EnvelopHandler) EnvelopHandler
	CommandMiddleware(command SlashCommand, next CommandHandler) CommandHandler
}

type EnvelopHandler func(Envelop, [][]string) ([]string, error)

type CommandHandler func(Envelop) (string, error)

type Script struct {
	Name             string                   `json:"name" gorm:"primary_key"`
	Description      string                   `json:"description"`
	Example          string                   `json:"example"`
	Matcher          string                   `json:"matcher"`
	TriggerOnMention bool                     `json:"trigger_on_mention"`
	Function         EnvelopHandler           `json:"-" gorm:"-"`
	Sanitizer        func(text string) string `json:"-" gorm:"-"`
	Type             TypeScript               `json:"type" gorm:"-"`
}

type TypeScript string

type Scripts []Script

func (s Scripts) ListFromType(typeScript TypeScript) Scripts {
	scripts := make([]Script, 0)
	for _, script := range s {
		if script.Type != typeScript {
			continue
		}
		scripts = append(scripts, script)
	}
	return Scripts(scripts)
}

func (s Script) String() string {
	return fmt.Sprintf("Script '%s' with matcher '%s' and type '%s'", s.Name, s.Matcher, s.Type)
}

type SlashCommand struct {
	Title       string         `json:"title" gorm:"primary_key"`
	Trigger     string         `json:"trigger_word"`
	Description string         `json:"description"`
	Function    CommandHandler `json:"-" gorm:"-"`
}

func (s SlashCommand) String() string {
	return fmt.Sprintf("Slash command '%s' with trigger word '%s' and type '%s'", s.Title, s.Trigger)
}
