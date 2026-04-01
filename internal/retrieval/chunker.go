package retrieval

import (
	"strings"
)

// ChunkByTokens splits text into chunks of up to maxTokens, with overlap tokens between consecutive chunks.
// It uses a simple paragraph aggregator and token estimator for stability.
func ChunkByTokens(text string, maxTokens, overlap int) []string {
	if maxTokens <= 0 {
		maxTokens = 400
	}
	if overlap < 0 {
		overlap = 0
	}
	paras := splitParagraphs(text)
	var chunks []string
	var window []string
	var curTokens int
	for _, p := range paras {
		t := approxTokens(p)
		if t > maxTokens {
			if len(window) > 0 {
				chunks = append(chunks, strings.Join(window, "\n\n"))
				if overlap > 0 {
					window, curTokens = backfillOverlap(window, overlap)
				} else {
					window = window[:0]
					curTokens = 0
				}
			}
			subs := hardSplitByTokens(p, maxTokens)
			chunks = append(chunks, subs...)
			continue
		}
		if curTokens+t > maxTokens && len(window) > 0 {
			chunks = append(chunks, strings.Join(window, "\n\n"))
			if overlap > 0 {
				window, curTokens = backfillOverlap(window, overlap)
			} else {
				window = window[:0]
				curTokens = 0
			}
		}
		window = append(window, p)
		curTokens += t
	}
	if len(window) > 0 {
		chunks = append(chunks, strings.Join(window, "\n\n"))
	}
	return chunks
}

func splitParagraphs(s string) []string {
	raw := strings.Split(s, "\n\n")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	if len(out) == 0 && strings.TrimSpace(s) != "" {
		return []string{strings.TrimSpace(s)}
	}
	return out
}

func backfillOverlap(paras []string, overlap int) ([]string, int) {
	var out []string
	tokens := 0
	for i := len(paras) - 1; i >= 0; i-- {
		t := approxTokens(paras[i])
		if tokens+t > overlap && len(out) > 0 {
			break
		}
		out = append([]string{paras[i]}, out...)
		tokens += t
	}
	return out, tokens
}

func hardSplitByTokens(s string, maxTokens int) []string {
	lines := strings.Split(s, "\n")
	var out []string
	var buf []string
	cur := 0
	for _, ln := range lines {
		lt := approxTokens(ln)
		if lt > maxTokens {
			if len(buf) > 0 {
				out = append(out, strings.Join(buf, "\n"))
				buf = nil
				cur = 0
			}
			out = append(out, splitByChars(ln, maxTokens*4)...)
			continue
		}
		if cur+lt > maxTokens && len(buf) > 0 {
			out = append(out, strings.Join(buf, "\n"))
			buf = nil
			cur = 0
		}
		buf = append(buf, ln)
		cur += lt
	}
	if len(buf) > 0 {
		out = append(out, strings.Join(buf, "\n"))
	}
	if len(out) == 0 {
		return splitByChars(s, maxTokens*4)
	}
	return out
}

func splitByChars(s string, charLimit int) []string {
	if charLimit <= 0 {
		return []string{s}
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return nil
	}
	var out []string
	for i := 0; i < len(r); i += charLimit {
		end := i + charLimit
		if end > len(r) {
			end = len(r)
		}
		out = append(out, string(r[i:end]))
	}
	return out
}

// approxTokens estimates tokens as 1 token ≈ 4 runes, without safety margin.
func approxTokens(s string) int {
	if s == "" {
		return 0
	}
	return len([]rune(s)) / 4
}
