package parser

import (
	"errors"
	"fmt"
	"os"

	"github.com/KaramelBytes/smushmux/internal/utils"
)

// Parser defines a document parser implementation.
type Parser interface {
	CanParse(filename string) bool
	Parse(content []byte) (string, error)
}

var registry []Parser

// Register adds a parser implementation to the registry.
func Register(p Parser) {
	registry = append(registry, p)
}

// ParseFile selects a parser based on filename and returns parsed text content.
func ParseFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	for _, p := range registry {
		if p.CanParse(path) {
			// Special-case parsers that need file path context
			switch tp := p.(type) {
			case csvParser:
				return ParseCSVFile(path)
			case xlsxParser:
				return ParseXLSXFile(path, "", 1)
			default:
				return tp.Parse(data)
			}
		}
	}
	// Fallback to plain text
	return string(data), nil
}

// EstimateTokens delegates to utils.CountTokens for now.
func EstimateTokens(text string) int {
	return utils.CountTokens(text)
}

func init() {
	// Register default parsers
	Register(txtParser{})
	Register(markdownParser{})
	Register(docxParser{})
	Register(csvParser{})
	Register(xlsxParser{})
}

// ErrUnsupported indicates a format is not supported yet.
var ErrUnsupported = errors.New("unsupported document format")
