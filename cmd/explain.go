package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/KaramelBytes/smushmux/internal/retrieval"
)

// retrievalEvidenceItem holds display data for a single retrieved chunk.
type retrievalEvidenceItem struct {
	Rank    int
	Score   float64
	DocName string
	ChunkID int
	Preview string // first ~200 runes of chunk text (truncated with "..." if longer)
}

// evidenceReport holds the structured data for the --explain output.
type evidenceReport struct {
	// Project
	ProjectName string
	ProjectPath string

	// Provider / model
	Provider    string
	Model       string
	ModelPreset string
	MaxTokens   int

	// Prompt stats
	PromptTokens int
	PromptLimit  int     // 0 = not set
	BudgetLimit  float64 // 0 = not set
	EstCost      float64

	// Retrieval
	RetrievalEnabled  bool
	RetrievalTopK     int
	RetrievalMinScore float64
	EmbedModel        string
	EmbedProvider     string
	MaxChunksPerDoc   int // 0 = no cap

	// Retrieval evidence (populated when retrieval is enabled and results are non-empty)
	RetrievalItems []retrievalEvidenceItem

	// Documents
	Documents []evidenceDocument
}

// evidenceDocument describes a single document included in the prompt.
type evidenceDocument struct {
	Name        string
	Description string
	Path        string
	Tokens      int
	IsDerived   bool // true for analysis artifacts (no path or synthetic content)
}

// buildEvidenceReport constructs an evidenceReport from the resolved run parameters.
func buildEvidenceReport(
	p *project.Project,
	projPath string,
	provider string,
	model string,
	modelPreset string,
	maxTokens int,
	promptTokens int,
	promptLimit int,
	budgetLimit float64,
	estCost float64,
	retrieval retrievalOptions,
	retrievalRecords []retrieval.ScoredRecord,
) evidenceReport {
	r := evidenceReport{
		ProjectName:       p.Name,
		ProjectPath:       projPath,
		Provider:          provider,
		Model:             model,
		ModelPreset:       modelPreset,
		MaxTokens:         maxTokens,
		PromptTokens:      promptTokens,
		PromptLimit:       promptLimit,
		BudgetLimit:       budgetLimit,
		EstCost:           estCost,
		RetrievalEnabled:  retrieval.Enabled,
		RetrievalTopK:     retrieval.TopK,
		RetrievalMinScore: retrieval.MinScore,
		EmbedModel:        retrieval.EmbedModel,
		EmbedProvider:     retrieval.EmbedProvider,
		MaxChunksPerDoc:   retrieval.MaxChunksPerDoc,
	}

	// Build retrieval evidence items (rank is 1-based; preview capped at 200 runes).
	const previewMaxRunes = 200
	for i, rec := range retrievalRecords {
		runes := []rune(rec.Text)
		preview := string(runes)
		if len(runes) > previewMaxRunes {
			preview = string(runes[:previewMaxRunes]) + "..."
		}
		r.RetrievalItems = append(r.RetrievalItems, retrievalEvidenceItem{
			Rank:    i + 1,
			Score:   rec.Score,
			DocName: rec.DocName,
			ChunkID: rec.ChunkID,
			Preview: preview,
		})
	}

	// Deterministic order for documents.
	ids := make([]string, 0, len(p.Documents))
	for id := range p.Documents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		d := p.Documents[id]
		r.Documents = append(r.Documents, evidenceDocument{
			Name:        d.Name,
			Description: d.Description,
			Path:        d.Path,
			Tokens:      d.Tokens,
			IsDerived:   d.Path == "",
		})
	}
	return r
}

// renderEvidenceReport writes a human-readable Markdown-style evidence report to w.
func renderEvidenceReport(r evidenceReport, w io.Writer) {
	fmt.Fprintln(w, "## Evidence Report")
	fmt.Fprintln(w)

	// ── Project ──────────────────────────────────────────────────────────────
	fmt.Fprintln(w, "### Project")
	fmt.Fprintf(w, "  Name : %s\n", r.ProjectName)
	fmt.Fprintf(w, "  Path : %s\n", r.ProjectPath)
	fmt.Fprintln(w)

	// ── Provider / Model ─────────────────────────────────────────────────────
	fmt.Fprintln(w, "### Provider / Model")
	fmt.Fprintf(w, "  Provider     : %s\n", nonEmpty(r.Provider, "(default)"))
	fmt.Fprintf(w, "  Model        : %s\n", r.Model)
	if r.ModelPreset != "" {
		fmt.Fprintf(w, "  Model Preset : %s\n", r.ModelPreset)
	}
	fmt.Fprintf(w, "  Max Tokens   : %d\n", r.MaxTokens)
	fmt.Fprintln(w)

	// ── Prompt Statistics ────────────────────────────────────────────────────
	fmt.Fprintln(w, "### Prompt Statistics")
	fmt.Fprintf(w, "  Prompt tokens (approx) : %d\n", r.PromptTokens)
	fmt.Fprintf(w, "  Max tokens requested   : %d\n", r.MaxTokens)
	if r.PromptLimit > 0 {
		fmt.Fprintf(w, "  Prompt limit           : %d tokens\n", r.PromptLimit)
	} else {
		fmt.Fprintf(w, "  Prompt limit           : (none)\n")
	}
	if r.BudgetLimit > 0 {
		fmt.Fprintf(w, "  Budget limit           : $%.4f\n", r.BudgetLimit)
	} else {
		fmt.Fprintf(w, "  Budget limit           : (none)\n")
	}
	if r.EstCost > 0 {
		fmt.Fprintf(w, "  Estimated max cost     : ~$%.4f\n", r.EstCost)
	}
	fmt.Fprintln(w)

	// ── Retrieval ────────────────────────────────────────────────────────────
	fmt.Fprintln(w, "### Retrieval")
	if r.RetrievalEnabled {
		fmt.Fprintf(w, "  Enabled        : yes\n")
		fmt.Fprintf(w, "  Top-K          : %d\n", r.RetrievalTopK)
		fmt.Fprintf(w, "  Min score      : %.2f\n", r.RetrievalMinScore)
		fmt.Fprintf(w, "  Embed model    : %s\n", nonEmpty(r.EmbedModel, "(default)"))
		fmt.Fprintf(w, "  Embed provider : %s\n", nonEmpty(r.EmbedProvider, "(default)"))
	} else {
		fmt.Fprintf(w, "  Enabled : no\n")
	}
	fmt.Fprintln(w)

	// ── Documents ────────────────────────────────────────────────────────────
	fmt.Fprintln(w, "### Documents Included in Prompt")
	if len(r.Documents) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for i, d := range r.Documents {
		label := d.Name
		if d.Description != "" {
			label = fmt.Sprintf("%s — %s", d.Name, d.Description)
		}
		source := d.Path
		if d.IsDerived {
			source = "(derived/analysis artifact)"
		}
		tags := []string{}
		if d.IsDerived {
			tags = append(tags, "derived")
		}
		tagStr := ""
		if len(tags) > 0 {
			tagStr = " [" + strings.Join(tags, ", ") + "]"
		}
		fmt.Fprintf(w, "  %d. %s%s\n", i+1, label, tagStr)
		fmt.Fprintf(w, "     Source : %s\n", source)
		fmt.Fprintf(w, "     Tokens : ~%d\n", d.Tokens)
	}

	// ── Retrieval evidence ───────────────────────────────────────────────────
	if len(r.RetrievalItems) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## Retrieval evidence")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  Top-K     : %d\n", r.RetrievalTopK)
		fmt.Fprintf(w, "  Min score : %.2f\n", r.RetrievalMinScore)
		if r.MaxChunksPerDoc > 0 {
			fmt.Fprintf(w, "  Max chunks/doc : %d\n", r.MaxChunksPerDoc)
		}
		fmt.Fprintln(w)
		for _, item := range r.RetrievalItems {
			fmt.Fprintf(w, "  %d. [score=%.4f] %s (chunk %d)\n", item.Rank, item.Score, item.DocName, item.ChunkID)
			fmt.Fprintf(w, "     Preview: %s\n", item.Preview)
		}
	}
}

// nonEmpty returns s if non-empty, otherwise fallback.
func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
