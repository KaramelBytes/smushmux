package utils_test

import (
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/utils"
)

func TestCountTokens(t *testing.T) {
	cases := []struct {
		name string
		in   string
		min  int
	}{
		{"empty", "", 0},
		{"simple", "hello world", 2},
		{"long", strings.Repeat("a", 4000), 900}, // heuristic ~ 1 tok ≈ 4 chars
	}
	for _, c := range cases {
		if got := utils.CountTokens(c.in); got < c.min {
			t.Errorf("%s: got %d < min %d", c.name, got, c.min)
		}
	}
}

func TestTruncateToTokenLimit(t *testing.T) {
	text := strings.Repeat("abcd ", 1000) // ~5000 chars
	trunc := utils.TruncateToTokenLimit(text, 300)
	n := utils.CountTokens(trunc)
	if n > 300 {
		t.Fatalf("tokens=%d exceeds limit", n)
	}
	if len(trunc) == 0 {
		t.Fatalf("expected non-empty truncation")
	}
}
