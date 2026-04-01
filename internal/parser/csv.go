package parser

import (
	"fmt"
	"strings"

	"github.com/KaramelBytes/smushmux/internal/analysis"
)

type csvParser struct{}

func (csvParser) CanParse(filename string) bool {
	name := strings.ToLower(filename)
	return strings.HasSuffix(name, ".csv") || strings.HasSuffix(name, ".tsv")
}

func (csvParser) Parse(_ []byte) (string, error) {
	// We don't use the content buffer here; ParseFile currently reads file and passes content.
	// For CSV we need the on-disk path. Refactor ParseFile to pass path in addition to content
	// would be ideal, but to keep compatibility, we re-open via a small hack: return an error
	// instructing the caller to use ParseFile for on-disk files.
	return "", fmt.Errorf("csv parser requires file path; use parser.ParseFile(path)")
}

// ParseCSVFile provides CSV parsing from an absolute file path to a compact summary.
func ParseCSVFile(path string) (string, error) {
	rep, err := analysis.AnalyzeCSV(path, analysis.DefaultOptions())
	if err != nil {
		return "", err
	}
	md := rep.Markdown()

	// Validate summary size before returning
	const maxSummaryChars = 100000 // ~20-30k tokens
	if len(md) > maxSummaryChars {
		// Provide detailed diagnostic
		return "", fmt.Errorf("CSV analysis produced %d character summary (limit: %d).\n"+
			"  File: %s\n"+
			"  Rows: %d, Columns: %d\n"+
			"  This file may be too large or complex.\n\n"+
			"Solutions:\n"+
			"  1. Use --max-rows <N> to limit rows analyzed (e.g., --max-rows 10000)\n"+
			"  2. Pre-filter the data to include only relevant rows/columns",
			len(md), maxSummaryChars, rep.Name, rep.Rows, len(rep.Cols))
	}

	return md, nil
}
