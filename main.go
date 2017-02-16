package main

import (
	"github.com/ArthurHlt/gubot/robot"

	// adapters
	_ "github.com/ArthurHlt/gubot/adapter/shell"
	_ "github.com/ArthurHlt/gubot/adapter/slack"
	//_ "github.com/ArthurHlt/gubot/adapter/mattermost_user"

	// scripts
	_ "github.com/ArthurHlt/gubot/scripts"

	"log"
	"os"
	"strconv"
)

func main() {
	port := 8080
	portStr := os.Getenv("PORT")
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}
	log.Fatal(robot.Start(port))
}
