package markdown

import (
	"fmt"
	"strings"
)

var NoSuchBlockError = fmt.Errorf("no such block")

func isHeading(l string) bool {
	return strings.HasPrefix(l, "#")
}

func headingLevel(l string) int {
	level := 0
	for _, char := range l {
		if char != '#' {
			break
		}
		level++
	}
	return level
}

func ExtractAnnotatedBlock(annotation string, s string) (string, error) {
	parts := strings.SplitN(s, "```"+annotation, 2)
	if len(parts) != 2 {
		return "", NoSuchBlockError
	}

	parts = strings.SplitN(parts[1], "```", 2)
	if len(parts) != 2 {
		return "", NoSuchBlockError
	}

	return strings.TrimSpace(parts[0]), nil
}

func ToListItem(lines string) []string {
	var result []string

	for i, line := range SplitLines(lines) {
		if i == 0 {
			result = append(result, "* "+line)
			continue
		}

		result = append(result, "  "+line)
	}

	return result
}

func SplitLines(s string) []string {
	return strings.Split(strings.Replace(s, `\r\n`, "\n", -1), "\n")
}
