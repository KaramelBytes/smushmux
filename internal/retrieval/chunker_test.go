package retrieval

import (
	"strings"
	"testing"
)

func makePara(letter string, tokens int) string {
	if letter == "" {
		letter = "x"
	}
	return strings.Repeat(letter, tokens*4)
}

func TestChunkByTokens_NoOverlap(t *testing.T) {
	p1 := makePara("a", 10)
	p2 := makePara("b", 10)
	p3 := makePara("c", 10)
	text := p1 + "\n\n" + p2 + "\n\n" + p3
	chunks := ChunkByTokens(text, 20, 0)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], p1) || !strings.Contains(chunks[0], p2) {
		t.Fatalf("chunk 0 should contain p1 and p2")
	}
	if strings.Contains(chunks[0], p3) {
		t.Fatalf("chunk 0 should not contain p3")
	}
	if !strings.Contains(chunks[1], p3) {
		t.Fatalf("chunk 1 should contain p3")
	}
}

func TestChunkByTokens_WithOverlap(t *testing.T) {
	p1 := makePara("a", 10)
	p2 := makePara("b", 10)
	p3 := makePara("c", 10)
	text := p1 + "\n\n" + p2 + "\n\n" + p3
	chunks := ChunkByTokens(text, 20, 5)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks with overlap, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], p1) || !strings.Contains(chunks[0], p2) {
		t.Fatalf("chunk 0 should contain p1 and p2")
	}
	if !strings.Contains(chunks[1], p2) || !strings.Contains(chunks[1], p3) {
		t.Fatalf("chunk 1 should contain p2 and p3 due to overlap")
	}
}
