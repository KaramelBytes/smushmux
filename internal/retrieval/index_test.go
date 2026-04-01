package retrieval

import (
	"context"
	"os"
	"strings"
	"testing"
)

type fakeEmbedder struct {
	dim   int
	calls int
}

func (f *fakeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	f.calls++
	if f.dim <= 0 {
		f.dim = 3
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, f.dim)
		v[i%f.dim] = 1.0
		out[i] = v
	}
	return out, nil
}

func TestBuildIndex_SaveAndReuse(t *testing.T) {
	dir := t.TempDir()
	// Two paragraphs -> two chunks with maxTokens=10
	p := strings.Repeat("a", 40) // ~10 tokens
	content := p + "\n\n" + p
	docs := map[string]struct{ Name, Content string }{
		"d1": {Name: "a.txt", Content: content},
	}
	emb := &fakeEmbedder{dim: 3}
	opts := BuildOptions{EmbedProvider: "openrouter", EmbedModel: "e1", ChunkMaxTokens: 10, ChunkOverlap: 0}
	idx, err := BuildIndex(context.Background(), emb, dir, docs, opts)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if idx.Meta.EmbedDim != 3 {
		t.Fatalf("expected EmbedDim=3, got %d", idx.Meta.EmbedDim)
	}
	if len(idx.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(idx.Records))
	}
	// index.json exists
	if _, err := os.Stat(IndexPath(dir)); err != nil {
		t.Fatalf("index.json not found: %v", err)
	}

	// Reuse: run again, expect no additional Embed calls
	emb2 := &fakeEmbedder{dim: 3}
	idx2, err := BuildIndex(context.Background(), emb2, dir, docs, opts)
	if err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	if len(idx2.Records) != 2 {
		t.Fatalf("expected 2 records on reuse, got %d", len(idx2.Records))
	}
	if emb2.calls != 0 {
		t.Fatalf("expected zero embed calls on reuse, got %d", emb2.calls)
	}
}

func TestBuildIndex_ForceRebuild(t *testing.T) {
	dir := t.TempDir()
	p := strings.Repeat("a", 40)
	content := p + "\n\n" + p
	docs := map[string]struct{ Name, Content string }{
		"d1": {Name: "a.txt", Content: content},
	}
	emb := &fakeEmbedder{dim: 3}
	opts := BuildOptions{EmbedProvider: "openrouter", EmbedModel: "e1", ChunkMaxTokens: 10, ChunkOverlap: 0}
	if _, err := BuildIndex(context.Background(), emb, dir, docs, opts); err != nil {
		t.Fatalf("first build: %v", err)
	}

	emb.calls = 0
	opts.Force = true
	_, err := BuildIndex(context.Background(), emb, dir, docs, opts)
	if err != nil {
		t.Fatalf("force rebuild: %v", err)
	}
	if emb.calls == 0 {
		t.Fatalf("expected embed to be called on force rebuild")
	}
}

func TestAllowDocFilters(t *testing.T) {
	if !allowDoc("report.txt", []string{"*.txt"}, nil) {
		t.Fatalf("expected include match")
	}
	if allowDoc("report.txt", []string{"*.md"}, nil) {
		t.Fatalf("unexpected include match")
	}
	if allowDoc("temp.log", nil, []string{"*.log"}) {
		t.Fatalf("exclude should filter out temp.log")
	}
}

func TestSearchRanking(t *testing.T) {
	dir := t.TempDir()
	// Three chunks -> one-hot vectors at indices 0,1,2
	p := strings.Repeat("a", 40)
	content := p + "\n\n" + p + "\n\n" + p
	docs := map[string]struct{ Name, Content string }{
		"d1": {Name: "a.txt", Content: content},
	}
	emb := &fakeEmbedder{dim: 3}
	opts := BuildOptions{EmbedProvider: "openrouter", EmbedModel: "e1", ChunkMaxTokens: 10, ChunkOverlap: 0}
	idx, err := BuildIndex(context.Background(), emb, dir, docs, opts)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if len(idx.Records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(idx.Records))
	}
	// Query aligns with the 2nd chunk vector (index 1)
	q := []float32{0, 1, 0}
	top := idx.Search(q, 1, 0.0)
	if len(top) != 1 {
		t.Fatalf("expected 1 top record, got %d", len(top))
	}
	if top[0].DocName != "a.txt" || top[0].ChunkID != 1 {
		t.Fatalf("expected doc a.txt chunk 1, got %s chunk %d", top[0].DocName, top[0].ChunkID)
	}
}

func TestIndexRoundtrip(t *testing.T) {
	dir := t.TempDir()
	// Minimal index
	idx := &Index{DocHashes: map[string]string{"d": "h"}, Records: []Record{{DocID: "d", DocName: "n", ChunkID: 0, Text: "x", Vector: []float32{1, 0}}}, Meta: IndexMeta{IndexVersion: 1}}
	p := IndexPath(dir)
	if err := idx.Save(p); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Records) != 1 || got.Records[0].DocID != "d" {
		t.Fatalf("roundtrip mismatch")
	}
}
