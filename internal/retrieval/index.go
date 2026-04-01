package retrieval

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

type Record struct {
	DocID     string    `json:"doc_id"`
	DocName   string    `json:"doc_name"`
	ChunkID   int       `json:"chunk_id"`
	ChunkHash string    `json:"chunk_hash,omitempty"`
	Text      string    `json:"text"`
	Vector    []float32 `json:"vector"`
}

type Index struct {
	// Map document id to content hash for invalidation
	DocHashes map[string]string `json:"doc_hashes"`
	Records   []Record          `json:"records"`
	Meta      IndexMeta         `json:"meta"`
}

type IndexMeta struct {
	IndexVersion   int       `json:"index_version"`
	EmbedProvider  string    `json:"embed_provider"`
	EmbedModel     string    `json:"embed_model"`
	EmbedDim       int       `json:"embed_dim"`
	ChunkMaxTokens int       `json:"chunk_max_tokens"`
	ChunkOverlap   int       `json:"chunk_overlap"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (idx *Index) Save(path string) error {
	if idx == nil {
		return fmt.Errorf("nil index")
	}
	if idx.DocHashes == nil {
		idx.DocHashes = map[string]string{}
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Load(path string) (*Index, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, err
	}
	if idx.DocHashes == nil {
		idx.DocHashes = map[string]string{}
	}
	// Backfill meta defaults for older indexes
	if idx.Meta.IndexVersion == 0 {
		idx.Meta.IndexVersion = 1
	}
	return &idx, nil
}

func IndexPath(projectRoot string) string {
	return filepath.Join(projectRoot, "index.json")
}

// metaCompatible checks if previous index metadata can be reused under current options.
func metaCompatible(prev, cur IndexMeta) bool {
	if prev.IndexVersion != cur.IndexVersion {
		return false
	}
	if prev.EmbedProvider != "" && cur.EmbedProvider != "" && prev.EmbedProvider != cur.EmbedProvider {
		return false
	}
	if prev.EmbedModel != "" && cur.EmbedModel != "" && prev.EmbedModel != cur.EmbedModel {
		return false
	}
	if prev.ChunkMaxTokens != 0 && cur.ChunkMaxTokens != 0 && prev.ChunkMaxTokens != cur.ChunkMaxTokens {
		return false
	}
	if prev.ChunkOverlap != 0 && cur.ChunkOverlap != 0 && prev.ChunkOverlap != cur.ChunkOverlap {
		return false
	}
	return true
}

// allowDoc filters by include/exclude patterns matched against doc name.
func allowDoc(name string, include, exclude []string) bool {
	matchAny := func(patterns []string) bool {
		for _, p := range patterns {
			if p == "" {
				continue
			}
			ok, _ := path.Match(p, name)
			if ok {
				return true
			}
		}
		return false
	}
	if len(include) > 0 && !matchAny(include) {
		return false
	}
	if len(exclude) > 0 && matchAny(exclude) {
		return false
	}
	return true
}

// Cosine similarity between two vectors. Returns 0 if dimensions mismatch.
func CosineSim(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	var na, nb float64
	for i := range a {
		fa := float64(a[i])
		fb := float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type BuildOptions struct {
	Force           bool
	EmbedProvider   string
	EmbedModel      string
	ChunkMaxTokens  int
	ChunkOverlap    int
	Include         []string
	Exclude         []string
	MaxChunksPerDoc int
}

// BuildIndex creates or refreshes the index for given documents.
// documents map key is doc id; value holds name and content.
func BuildIndex(ctx context.Context, emb Embedder, projectRoot string, documents map[string]struct{ Name, Content string }, opts BuildOptions) (*Index, error) {
	path := IndexPath(projectRoot)
	prev, _ := Load(path) // best effort
	if prev == nil {
		prev = &Index{DocHashes: map[string]string{}}
	}
	// Prepare fresh index state
	idx := &Index{DocHashes: map[string]string{}, Records: nil}
	// Defaults
	if opts.ChunkMaxTokens <= 0 {
		opts.ChunkMaxTokens = 400
	}
	if opts.ChunkOverlap < 0 {
		opts.ChunkOverlap = 0
	}
	idx.Meta = IndexMeta{
		IndexVersion:   1,
		EmbedProvider:  opts.EmbedProvider,
		EmbedModel:     opts.EmbedModel,
		EmbedDim:       0, // set after embed
		ChunkMaxTokens: opts.ChunkMaxTokens,
		ChunkOverlap:   opts.ChunkOverlap,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	type work struct {
		docID, docName string
		chunks         []string
	}
	var allChunks []work
	for id, d := range documents {
		if !allowDoc(d.Name, opts.Include, opts.Exclude) {
			continue
		}
		chunks := ChunkByTokens(d.Content, opts.ChunkMaxTokens, opts.ChunkOverlap)
		if opts.MaxChunksPerDoc > 0 && len(chunks) > opts.MaxChunksPerDoc {
			chunks = chunks[:opts.MaxChunksPerDoc]
		}
		allChunks = append(allChunks, work{docID: id, docName: d.Name, chunks: chunks})
		sum := sha1.Sum([]byte(d.Content))
		idx.DocHashes[id] = fmt.Sprintf("%x", sum[:])
	}
	// Build reuse map from previous index
	byDoc := map[string][]Record{}
	for _, r := range prev.Records {
		byDoc[r.DocID] = append(byDoc[r.DocID], r)
	}
	// Compute chunk hashes and decide which chunks need embedding
	type chunkMeta struct {
		docID, docName string
		chunkID        int
		text, hash     string
	}
	var toEmbed []chunkMeta
	var reuse []Record
	for _, w := range allChunks {
		prevChunks := byDoc[w.docID]
		for i, text := range w.chunks {
			h := sha1.Sum([]byte(text))
			ch := fmt.Sprintf("%x", h[:])
			var matched *Record
			if !opts.Force && metaCompatible(prev.Meta, idx.Meta) {
				for j := range prevChunks {
					pr := prevChunks[j]
					if pr.ChunkID == i && pr.ChunkHash == ch && len(pr.Vector) > 0 {
						matched = &pr
						break
					}
				}
			}
			if matched != nil {
				reuse = append(reuse, *matched)
			} else {
				toEmbed = append(toEmbed, chunkMeta{docID: w.docID, docName: w.docName, chunkID: i, text: text, hash: ch})
			}
		}
	}
	// If nothing to embed and we can reuse, just return reuse records
	if len(toEmbed) == 0 {
		idx.Records = append(idx.Records, reuse...)
		sort.Slice(idx.Records, func(i, j int) bool {
			if idx.Records[i].DocName == idx.Records[j].DocName {
				return idx.Records[i].ChunkID < idx.Records[j].ChunkID
			}
			return idx.Records[i].DocName < idx.Records[j].DocName
		})
		if err := idx.Save(path); err != nil {
			return nil, err
		}
		return idx, nil
	}
	// Embed required chunks in batches
	const maxEmbedBatchSize = 100 // Conservative batch size
	vecs := make([][]float32, 0, len(toEmbed))

	fmt.Printf("Embedding %d chunks in batches of %d...\n", len(toEmbed), maxEmbedBatchSize)

	for start := 0; start < len(toEmbed); start += maxEmbedBatchSize {
		end := start + maxEmbedBatchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}

		batchToEmbed := toEmbed[start:end]
		chunkTexts := make([]string, len(batchToEmbed))
		for i, cm := range batchToEmbed {
			chunkTexts[i] = cm.text
		}

		fmt.Printf("  Processing batch %d-%d...\n", start+1, end)

		batchVecs, err := emb.Embed(ctx, chunkTexts)
		if err != nil {
			return nil, fmt.Errorf("embed batch %d-%d: %w", start, end, err)
		}

		vecs = append(vecs, batchVecs...)

		// Allow brief GC opportunity between batches
		if end < len(toEmbed) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	fmt.Printf("✓ Embedded %d chunks successfully\n", len(vecs))

	// Assemble final records
	idx.Records = append(idx.Records, reuse...)
	for i := range toEmbed {
		if i >= len(vecs) {
			break
		}
		cm := toEmbed[i]
		r := Record{DocID: cm.docID, DocName: cm.docName, ChunkID: cm.chunkID, ChunkHash: cm.hash, Text: cm.text, Vector: vecs[i]}
		idx.Records = append(idx.Records, r)
	}
	if len(vecs) > 0 && len(vecs[0]) > 0 {
		idx.Meta.EmbedDim = len(vecs[0])
	}
	// Deterministic order (by doc name, then chunk id)
	sort.Slice(idx.Records, func(i, j int) bool {
		if idx.Records[i].DocName == idx.Records[j].DocName {
			return idx.Records[i].ChunkID < idx.Records[j].ChunkID
		}
		return idx.Records[i].DocName < idx.Records[j].DocName
	})
	if err := idx.Save(path); err != nil {
		return nil, err
	}
	return idx, nil
}

// ScoredRecord is a Record paired with its cosine-similarity score from a search query.
type ScoredRecord struct {
	Record
	Score float64
}

// SearchWithScores returns top-k records above minScore, sorted by descending score.
// The Score field on each returned item reflects the cosine similarity to the query.
func (idx *Index) SearchWithScores(query []float32, topK int, minScore float64) []ScoredRecord {
	scored := make([]ScoredRecord, 0, len(idx.Records))
	for _, r := range idx.Records {
		s := CosineSim(query, r.Vector)
		if s >= minScore {
			scored = append(scored, ScoredRecord{Record: r, Score: s})
		}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	if topK > 0 && len(scored) > topK {
		scored = scored[:topK]
	}
	return scored
}

// Search returns top-k records above the minScore threshold, sorted by descending score.
func (idx *Index) Search(query []float32, topK int, minScore float64) []Record {
	scored := idx.SearchWithScores(query, topK, minScore)
	out := make([]Record, len(scored))
	for i, s := range scored {
		out[i] = s.Record
	}
	return out
}
