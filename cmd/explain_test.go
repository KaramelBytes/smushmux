package cmd

import (
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/KaramelBytes/smushmux/internal/retrieval"
)

func sampleProject() *project.Project {
	p := &project.Project{
		Name:         "testproj",
		Instructions: "Summarize",
		Documents: map[string]*project.Document{
			"aaa": {
				ID:          "aaa",
				Name:        "file1.txt",
				Description: "main report",
				Path:        "/docs/file1.txt",
				Tokens:      120,
			},
			"bbb": {
				ID:          "bbb",
				Name:        "derived.csv",
				Description: "analysis output",
				Path:        "",
				Tokens:      45,
			},
		},
	}
	return p
}

func TestRenderEvidenceReport_Headings(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/home/user/.smushmux/projects/testproj",
		"openrouter", "openai/gpt-4o-mini", "", 1024,
		500, 0, 0, 0,
		retrievalOptions{Enabled: false},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	for _, want := range []string{
		"## Evidence Report",
		"### Project",
		"### Provider / Model",
		"### Prompt Statistics",
		"### Retrieval",
		"### Documents Included in Prompt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing heading %q in output:\n%s", want, out)
		}
	}
}

func TestRenderEvidenceReport_ProjectInfo(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/some/path",
		"ollama", "llama3", "", 512,
		200, 0, 0, 0,
		retrievalOptions{Enabled: false},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	if !strings.Contains(out, "testproj") {
		t.Errorf("project name missing in output:\n%s", out)
	}
	if !strings.Contains(out, "/some/path") {
		t.Errorf("project path missing in output:\n%s", out)
	}
}

func TestRenderEvidenceReport_ProviderAndModel(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"openrouter", "openai/gpt-4o-mini", "balanced", 1024,
		300, 0, 0, 0,
		retrievalOptions{Enabled: false},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	if !strings.Contains(out, "openrouter") {
		t.Errorf("provider 'openrouter' missing in output:\n%s", out)
	}
	if !strings.Contains(out, "openai/gpt-4o-mini") {
		t.Errorf("model missing in output:\n%s", out)
	}
	if !strings.Contains(out, "balanced") {
		t.Errorf("model preset 'balanced' missing in output:\n%s", out)
	}
}

func TestRenderEvidenceReport_PromptStats(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"", "openai/gpt-4o-mini", "", 512,
		800, 5000, 0.05, 0.001,
		retrievalOptions{Enabled: false},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	if !strings.Contains(out, "800") {
		t.Errorf("prompt tokens missing in output:\n%s", out)
	}
	if !strings.Contains(out, "5000") {
		t.Errorf("prompt limit missing in output:\n%s", out)
	}
	if !strings.Contains(out, "0.0500") {
		t.Errorf("budget limit missing in output:\n%s", out)
	}
}

func TestRenderEvidenceReport_RetrievalEnabled(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"openrouter", "openai/gpt-4o-mini", "", 1024,
		400, 0, 0, 0,
		retrievalOptions{
			Enabled:       true,
			TopK:          8,
			MinScore:      0.25,
			EmbedModel:    "nomic-embed-text",
			EmbedProvider: "ollama",
		},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	if !strings.Contains(out, "yes") {
		t.Errorf("retrieval 'yes' missing in output:\n%s", out)
	}
	if !strings.Contains(out, "8") {
		t.Errorf("top-k missing in output:\n%s", out)
	}
	if !strings.Contains(out, "nomic-embed-text") {
		t.Errorf("embed model missing in output:\n%s", out)
	}
	if !strings.Contains(out, "ollama") {
		t.Errorf("embed provider missing in output:\n%s", out)
	}
}

func TestRenderEvidenceReport_RetrievalDisabled(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"", "openai/gpt-4o-mini", "", 1024,
		300, 0, 0, 0,
		retrievalOptions{Enabled: false},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	if !strings.Contains(out, "no") {
		t.Errorf("retrieval 'no' missing in output:\n%s", out)
	}
}

func TestRenderEvidenceReport_Documents(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"", "openai/gpt-4o-mini", "", 1024,
		300, 0, 0, 0,
		retrievalOptions{Enabled: false},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	// Documents should be listed deterministically (sorted by ID: aaa < bbb)
	if !strings.Contains(out, "file1.txt") {
		t.Errorf("doc 'file1.txt' missing in output:\n%s", out)
	}
	if !strings.Contains(out, "/docs/file1.txt") {
		t.Errorf("doc path missing in output:\n%s", out)
	}
	if !strings.Contains(out, "derived.csv") {
		t.Errorf("doc 'derived.csv' missing in output:\n%s", out)
	}
	// derived doc should show as derived artifact
	if !strings.Contains(out, "derived/analysis artifact") {
		t.Errorf("derived artifact label missing in output:\n%s", out)
	}
}

func TestRenderEvidenceReport_DefaultProviderLabel(t *testing.T) {
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"", "openai/gpt-4o-mini", "", 1024,
		300, 0, 0, 0,
		retrievalOptions{Enabled: false},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	if !strings.Contains(out, "(default)") {
		t.Errorf("expected '(default)' label for empty provider, got:\n%s", out)
	}
}

func TestBuildEvidenceReport_DocumentOrder(t *testing.T) {
	// Verify documents appear in deterministic sorted order (by map key).
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"", "model", "", 0,
		0, 0, 0, 0,
		retrievalOptions{},
		nil,
	)
	if len(r.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(r.Documents))
	}
	// "aaa" sorts before "bbb"
	if r.Documents[0].Name != "file1.txt" {
		t.Errorf("expected first doc 'file1.txt' (key aaa), got %q", r.Documents[0].Name)
	}
	if r.Documents[1].Name != "derived.csv" {
		t.Errorf("expected second doc 'derived.csv' (key bbb), got %q", r.Documents[1].Name)
	}
}

func TestNonEmpty(t *testing.T) {
	if got := nonEmpty("hello", "fallback"); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	if got := nonEmpty("", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}
}

// sampleScoredRecords returns a deterministic list of two scored retrieval records.
func sampleScoredRecords() []retrieval.ScoredRecord {
	return []retrieval.ScoredRecord{
		{
			Record: retrieval.Record{DocName: "report.pdf", ChunkID: 2, Text: "Q3 revenue increased."},
			Score:  0.8523,
		},
		{
			Record: retrieval.Record{DocName: "balance.xlsx", ChunkID: 0, Text: "Total assets Q3."},
			Score:  0.7941,
		},
	}
}

func TestRenderEvidenceReport_RetrievalEvidenceSection(t *testing.T) {
	p := sampleProject()
	recs := sampleScoredRecords()
	r := buildEvidenceReport(p, "/path",
		"openrouter", "openai/gpt-4o-mini", "", 1024,
		400, 0, 0, 0,
		retrievalOptions{Enabled: true, TopK: 6, MinScore: 0.20},
		recs,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()

	if !strings.Contains(out, "## Retrieval evidence") {
		t.Errorf("missing '## Retrieval evidence' heading in output:\n%s", out)
	}
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("missing doc name 'report.pdf' in retrieval evidence:\n%s", out)
	}
	if !strings.Contains(out, "balance.xlsx") {
		t.Errorf("missing doc name 'balance.xlsx' in retrieval evidence:\n%s", out)
	}
	if !strings.Contains(out, "0.8523") {
		t.Errorf("missing score '0.8523' in retrieval evidence:\n%s", out)
	}
	if !strings.Contains(out, "chunk 2") {
		t.Errorf("missing chunk id '2' in retrieval evidence:\n%s", out)
	}
	if !strings.Contains(out, "Q3 revenue increased.") {
		t.Errorf("missing chunk preview in retrieval evidence:\n%s", out)
	}
}

func TestRenderEvidenceReport_RetrievalEvidenceDeterministicOrder(t *testing.T) {
	// Records arrive ranked (highest score first) from SearchWithScores.
	// The evidence section must preserve that order.
	p := sampleProject()
	recs := sampleScoredRecords() // score[0]=0.8523 > score[1]=0.7941
	r := buildEvidenceReport(p, "/path",
		"", "model", "", 0, 0, 0, 0, 0,
		retrievalOptions{Enabled: true, TopK: 6},
		recs,
	)
	if len(r.RetrievalItems) != 2 {
		t.Fatalf("expected 2 retrieval items, got %d", len(r.RetrievalItems))
	}
	if r.RetrievalItems[0].DocName != "report.pdf" {
		t.Errorf("expected first item 'report.pdf', got %q", r.RetrievalItems[0].DocName)
	}
	if r.RetrievalItems[1].DocName != "balance.xlsx" {
		t.Errorf("expected second item 'balance.xlsx', got %q", r.RetrievalItems[1].DocName)
	}
	if r.RetrievalItems[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", r.RetrievalItems[0].Rank)
	}
	if r.RetrievalItems[1].Rank != 2 {
		t.Errorf("expected rank 2, got %d", r.RetrievalItems[1].Rank)
	}
}

func TestBuildEvidenceReport_PreviewTrimming(t *testing.T) {
	// Chunk text longer than 200 runes should be trimmed with "..." appended.
	longText := strings.Repeat("abcde", 50) // 250 runes
	p := sampleProject()
	recs := []retrieval.ScoredRecord{
		{
			Record: retrieval.Record{DocName: "doc.txt", ChunkID: 0, Text: longText},
			Score:  0.9,
		},
	}
	r := buildEvidenceReport(p, "/path",
		"", "model", "", 0, 0, 0, 0, 0,
		retrievalOptions{Enabled: true},
		recs,
	)
	if len(r.RetrievalItems) != 1 {
		t.Fatalf("expected 1 retrieval item, got %d", len(r.RetrievalItems))
	}
	preview := r.RetrievalItems[0].Preview
	if !strings.HasSuffix(preview, "...") {
		t.Errorf("expected preview to end with '...', got %q", preview)
	}
	runeCount := len([]rune(preview))
	// 200 runes + 3 runes for "..."
	if runeCount != 203 {
		t.Errorf("expected preview length 203 runes (200 + '...'), got %d", runeCount)
	}
}

func TestBuildEvidenceReport_PreviewNoTrimShortText(t *testing.T) {
	// Chunk text under 200 runes should not be trimmed.
	shortText := "Short text."
	p := sampleProject()
	recs := []retrieval.ScoredRecord{
		{
			Record: retrieval.Record{DocName: "doc.txt", ChunkID: 0, Text: shortText},
			Score:  0.5,
		},
	}
	r := buildEvidenceReport(p, "/path",
		"", "model", "", 0, 0, 0, 0, 0,
		retrievalOptions{Enabled: true},
		recs,
	)
	if r.RetrievalItems[0].Preview != shortText {
		t.Errorf("expected preview %q, got %q", shortText, r.RetrievalItems[0].Preview)
	}
}

func TestRenderEvidenceReport_RetrievalEvidenceAbsentWhenEmpty(t *testing.T) {
	// When no retrieval results exist, the ## Retrieval evidence section must not appear.
	p := sampleProject()
	r := buildEvidenceReport(p, "/path",
		"", "model", "", 0, 0, 0, 0, 0,
		retrievalOptions{Enabled: true, TopK: 6},
		nil,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()
	if strings.Contains(out, "## Retrieval evidence") {
		t.Errorf("unexpected '## Retrieval evidence' section when results are empty:\n%s", out)
	}
}

func TestRenderEvidenceReport_MaxChunksPerDoc(t *testing.T) {
	// MaxChunksPerDoc > 0 should appear in the Retrieval evidence section.
	p := sampleProject()
	recs := sampleScoredRecords()
	r := buildEvidenceReport(p, "/path",
		"", "model", "", 0, 0, 0, 0, 0,
		retrievalOptions{Enabled: true, TopK: 6, MaxChunksPerDoc: 3},
		recs,
	)
	var buf strings.Builder
	renderEvidenceReport(r, &buf)
	out := buf.String()
	if !strings.Contains(out, "Max chunks/doc") {
		t.Errorf("expected 'Max chunks/doc' in output when MaxChunksPerDoc=3:\n%s", out)
	}
}
