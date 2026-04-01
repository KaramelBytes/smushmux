package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/parser"
)

func TestParseFileCSV_Summary(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hop_harvest.csv")
	content := "date,plot,alpha_acids,moisture\n" +
		"2024-08-10,A1,12.5%,74\n" +
		"2024-08-12,A1,11.8%,71\n" +
		"2024-08-15,B3,10.2%,68\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := parser.ParseFile(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(out, "[DATASET SUMMARY]") {
		t.Fatalf("expected dataset summary header, got: %q", out)
	}
	if !strings.Contains(out, "alpha_acids [%]: numeric") {
		t.Fatalf("expected numeric inference with percent unit for alpha_acids, got: %q", out)
	}
	if !strings.Contains(out, "date: datetime") {
		t.Fatalf("expected datetime inference for date, got: %q", out)
	}
}
