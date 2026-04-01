package parser

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type docxParser struct{}

func (docxParser) CanParse(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".docx")
}

func (docxParser) Parse(content []byte) (string, error) {
	// DOCX is a zip archive; extract word/document.xml and strip XML tags as a naive text extraction
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	var docXML []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open document.xml: %w", err)
			}
			b, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return "", fmt.Errorf("read document.xml: %w", err)
			}
			docXML = b
			break
		}
	}
	if len(docXML) == 0 {
		return "", fmt.Errorf("document.xml not found in DOCX")
	}
	// Remove XML tags. This is simplistic but OK for MVP.
	re := regexp.MustCompile(`<[^>]+>`) // matches tags
	text := re.ReplaceAllString(string(docXML), "")
	// Normalize whitespace
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text, nil
}
