package parser

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/KaramelBytes/smushmux/internal/analysis"
)

type xlsxParser struct{}

func (xlsxParser) CanParse(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".xlsx")
}

func (xlsxParser) Parse(_ []byte) (string, error) {
	return "", fmt.Errorf("xlsx parser requires file path; use parser.ParseFile(path)")
}

// ParseXLSXFile analyzes the first sheet and returns a compact summary.
func ParseXLSXFile(path string, sheetName string, sheetIndex int) (string, error) {
	rep, err := analysis.AnalyzeXLSX(path, analysis.DefaultOptions(), sheetName, sheetIndex)
	if err != nil {
		return "", err
	}
	// Optionally, include sheet in name for clarity
	if rep != nil && rep.Name == filepath.Base(path) && sheetName != "" {
		rep.Name = fmt.Sprintf("%s (sheet: %s)", rep.Name, sheetName)
	}
	md := rep.Markdown()

	// Validate summary size before returning
	const maxSummaryChars = 100000 // ~20-30k tokens
	if len(md) > maxSummaryChars {
		// Provide detailed diagnostic
		return "", fmt.Errorf("XLSX analysis produced %d character summary (limit: %d).\n"+
			"  File: %s\n"+
			"  Rows: %d, Columns: %d\n"+
			"  This file may be too large or complex.\n\n"+
			"Solutions:\n"+
			"  1. Use --max-rows <N> to limit rows analyzed (e.g., --max-rows 10000)\n"+
			"  2. Analyze specific sheet with --sheet-name if workbook has multiple sheets\n"+
			"  3. Pre-filter the data to include only relevant rows/columns",
			len(md), maxSummaryChars, filepath.Base(path), rep.Rows, len(rep.Cols))
	}

	return md, nil
}
