package parser

import (
	"bytes"
	"strings"
)

type markdownParser struct{}

func (markdownParser) CanParse(filename string) bool {
	name := strings.ToLower(filename)
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown")
}

func (markdownParser) Parse(content []byte) (string, error) {
	// MVP: Preserve content as-is, normalize line endings and trim excessive blank lines.
	text := string(bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n")))
	text = strings.ReplaceAll(text, "\r", "\n")
	// Collapse >2 consecutive newlines to exactly two
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text, nil
}
