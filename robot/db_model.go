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
	Name  string     `json:"name" gorm:"primary_key"`
	Regex string     `json:"regex"`
	Url   string     `json:"url"`
	Type  string     `json:"type"`
}