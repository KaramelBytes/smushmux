package analysis

import (
	"testing"
)

// TestXLSXRelationshipPathNormalization verifies that XLSX files with
// relationship targets using leading slashes (e.g., "/xl/worksheets/sheet1.xml")
// are correctly parsed. This was a regression where the parser failed to read
// sheets because it didn't strip the leading slash before constructing the ZIP path.
//
// The embedded test fixture in table_test.go contains relationships with various
// path formats to ensure the normalizeRelPath function handles them correctly.
func TestXLSXRelationshipPathNormalization(t *testing.T) {
	opt := DefaultOptions()
	opt.SampleRows = 2
	opt.MaxRows = 10
	
	// The test will use the fixture from table_test.go via TestAnalyzeXLSXSheetSelectionAndMarkdown
	// Here we just verify the normalizeRelPath helper directly
	t.Run("normalizeRelPath", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"/xl/worksheets/sheet1.xml", "xl/worksheets/sheet1.xml"},
			{"xl/worksheets/sheet1.xml", "xl/worksheets/sheet1.xml"},
			{"/worksheets/sheet1.xml", "xl/worksheets/sheet1.xml"},
			{"worksheets/sheet1.xml", "xl/worksheets/sheet1.xml"},
			{"styles.xml", "xl/styles.xml"},
			{"/xl/styles.xml", "xl/styles.xml"},
		}
		
		for _, tt := range tests {
			got := normalizeRelPath(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeRelPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		}
	})
}

