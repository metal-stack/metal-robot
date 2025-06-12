package markdown

import (
	"fmt"
	"strings"
)

var ErrNoSuchBlock = fmt.Errorf("no such block")

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
	_, contentWithTicks, found := strings.Cut(s, "```"+annotation)
	if !found {
		return "", ErrNoSuchBlock
	}

	content, _, found := strings.Cut(contentWithTicks, "```")
	if !found {
		return "", ErrNoSuchBlock
	}

	return strings.TrimSpace(content), nil
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
	return strings.Split(strings.ReplaceAll(s, `\r\n`, "\n"), "\n")
}
