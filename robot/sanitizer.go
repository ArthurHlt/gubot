package robot

import (
	"strings"
	"regexp"
)

func NoSanitize(text string) string {
	return text
}
func SanitizeDefault(text string) string {
	return SanitizeSpaces(SanitizeNewLine(SanitizeTab(text)))
}
func SanitizeSpaces(text string) string {
	return sanitizeGeneric(text, "  ", " ")
}
func SanitizeNewLine(text string) string {
	return sanitizeGeneric(text, "\n", " ")
}
func SanitizeTab(text string) string {
	return sanitizeGeneric(text, "\t", " ")
}
func sanitizeGeneric(text, substring, replacement string) string {
	if !strings.Contains(text, substring) {
		return text
	}
	return sanitizeGeneric(strings.Replace(text, substring, replacement, -1), substring, replacement)
}
func SanitizeDefaultWithSpecialChar(text string) string {
	r := regexp.MustCompile("[^(\\w|\\s)]")
	return SanitizeDefault(r.ReplaceAllString(text, ""))
}
