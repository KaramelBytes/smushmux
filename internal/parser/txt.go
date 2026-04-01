package parser

import "strings"

type txtParser struct{}

func (txtParser) CanParse(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".txt")
}

func (txtParser) Parse(content []byte) (string, error) {
	return string(content), nil
}
