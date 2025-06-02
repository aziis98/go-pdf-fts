package util

import "strings"

// Dedent removes leading and trailing whitespace from each line, also trims any initial and trailing whitespace from the entire string.
func Dedent(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}
