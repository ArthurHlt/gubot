package main

import (
	"github.com/ArthurHlt/gubot/robot"

	// adapters
	//_ "github.com/ArthurHlt/gubot/adapter/shell"
	//_ "github.com/ArthurHlt/gubot/adapter/slack"
	_ "github.com/ArthurHlt/gubot/adapter/mattermost_user"

	// scripts
	_ "github.com/ArthurHlt/gubot/scripts"

	"log"
	"os"
)

func main() {
	addr := ":8080"
	port := os.Getenv("PORT")
	if port != "" {
		addr = ":" + port
	}
	log.Fatal(robot.Start(addr))
}
