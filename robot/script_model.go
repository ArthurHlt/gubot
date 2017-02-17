package robot

const (
	Tsend TypeScript = "send"
	Trespond TypeScript = "respond"
)

type Script struct {
	Name             string `json:"name" gorm:"primary_key"`
	Description      string `json:"description"`
	Example          string `json:"example"`
	Matcher          string `json:"matcher"`
	TriggerOnMention bool   `json:"trigger_on_mention"`
	Function         func(Envelop, [][]string) ([]string, error) `json:"-" gorm:"-"`
	Sanitizer        func(text string) string `json:"-" gorm:"-"`
	Type             TypeScript `json:"type" gorm:"-"`
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