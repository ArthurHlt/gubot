package adapter

import "strings"

func TruncateMessage(message string, sizeMax int) []string {
	message = strings.TrimSpace(message)
	if len(message) <= sizeMax {
		return []string{message}
	}
	allMessages := make([]string, 0)
	index := strings.LastIndex(message[0:sizeMax], "\n")
	if index == -1 {
		index = strings.LastIndex(message[0:sizeMax], " ")
	}
	if index == -1 || index < 2 {
		index = sizeMax
	}
	allMessages = append(allMessages, strings.TrimSpace(message[0:index]))
	allMessages = append(allMessages, TruncateMessage(message[index:], sizeMax)...)

	return allMessages
}
