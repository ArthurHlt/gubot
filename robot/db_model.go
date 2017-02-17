package robot

import "github.com/jinzhu/gorm"

type User struct {
	gorm.Model
	UserId  string `gorm:"primary_key"`
	Name    string `gorm:"primary_key"`
	Channel string
}
type RemoteScript struct {
	gorm.Model
	Script
	Url  string `json:"url"`
	Type string `json:"type"`
}

func (r RemoteScript) ToScript() Script {
	return Script{
		Name: r.Name,
		Description: r.Description,
		Example: r.Example,
		Matcher: r.Matcher,
		TriggerOnMention: r.TriggerOnMention,
		Type: TypeScript(r.Type),
	}
}