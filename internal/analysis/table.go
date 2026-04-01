package analysis

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Options controls analysis behavior for tabular data.
type Options struct {
	// MaxRows limits rows processed; 0 means unlimited.
	MaxRows int
	// MaxColumns limits the number of columns processed; 0 means unlimited.
	MaxColumns int
	// SampleRows determines how many example rows to include in the report.
	SampleRows int
	// Delimiter for CSV. If 0, auto-detects among ',', ';', '\t'.
	Delimiter rune
	// GroupBy computes per-group summaries for the given column names.
	GroupBy []string
	// Correlations computes Pearson correlations among numeric columns.
	Correlations bool
	// CorrPerGroup computes correlations per group key.
	CorrPerGroup bool
	// Numeric parsing locale. If DecimalSeparator is 0, auto-detect per value.
	DecimalSeparator   rune
	ThousandsSeparator rune // optional; if 0, auto-detect common separators (',' '.' space)
	// Outlier detection via robust Z-score (MAD). If Outliers is true, counts |z|>threshold.
	Outliers         bool
	OutlierThreshold float64
	// Unit normalization: convert values to target units using simple mappings.
	UnitNormalize bool
	UnitTargets   map[string]string // map[fromUnit]toUnit, e.g., {"g/L":"mg/L", "ug/L":"mg/L", "°F":"°C"}
}

// DefaultOptions returns reasonable defaults for dataset analysis.
func DefaultOptions() Options {
	return Options{
		MaxRows:       1000,
		MaxColumns:    50,
		SampleRows:    5,
		UnitNormalize: true,
		UnitTargets: map[string]string{
			"g/L":  "mg/L",
			"ug/L": "mg/L",
			"°F":   "°C",
		},
	}
}

// Report is a markdown-friendly analysis of a tabular dataset.
type Report struct {
	Name      string
	Rows      int
	Processed int
	Cols      []ColumnSummary
	Samples   [][]string
	Warnings  []string
	Groups    []GroupResult
	Corr      *CorrMatrix
}

// ColumnSummary captures inferred type and statistics per column.
type ColumnSummary struct {
	Name    string
	Kind    string // numeric|datetime|categorical|text|unknown
	Unit    string
	NonNull int
	Missing int
	Unique  int
	// Numeric stats
	Min  float64
	Max  float64
	Mean float64
	Std  float64
	// Outliers (robust Z via MAD)
	OutliersCount    int
	OutliersMaxAbsZ  float64
	OutlierThreshold float64
	// Categorical top values
	TopValues    []CategoryCount
	ExampleTexts []string
}

type CategoryCount struct {
	Value string
	Count int
}

// GroupResult captures aggregated metrics per group key.
type GroupResult struct {
	Key       string
	Size      int
	Metrics   map[string]NumSummary // by column name
	CorrPairs []PairCorr            // top correlation pairs (by |r|)
}

type NumSummary struct {
	Count          int
	Min, Max, Mean float64
}

// CorrMatrix holds a symmetric Pearson correlation matrix across numeric columns.
type CorrMatrix struct {
	Columns []string
	Values  [][]float64 // row-major, Values[i][j]
}

// PairCorr is a simple correlation pair summary.
type PairCorr struct {
	A, B string
	R    float64
}

// AnalyzeCSV analyzes a CSV file and returns a Report.
func AnalyzeCSV(path string, opt Options) (*Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()
	// Sniff delimiter
	delim := opt.Delimiter
	if delim == 0 {

		delim = sniffDelimiter(path)

	}
	r := csv.NewReader(f)
	r.ReuseRecord = true
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	r.Comma = delim

	// Read header
	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return &Report{Name: filepath.Base(path)}, nil
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	var droppedCols int
	if opt.MaxColumns > 0 && len(header) > opt.MaxColumns {
		droppedCols = len(header) - opt.MaxColumns
		header = header[:opt.MaxColumns]
	}

	ncol := len(header)
	if ncol == 0 {
		return &Report{Name: filepath.Base(path)}, nil
	}

	// Per-column accumulators
	type colAcc struct {

		name     string
		unit     string
		origUnit string
		nonNil   int
		miss     int

		// numeric stats via Welford
		n      int
		mean   float64
		m2     float64
		min    float64
		max    float64
		numCnt int
		dtCnt  int
		txtCnt int
		cats   map[string]int
		exText []string
	}
	cols := make([]*colAcc, ncol)
	// Map for group-by index lookup
	gbIndex := map[string]int{}
	for i := range header {
		hn := strings.TrimSpace(header[i])
		clean, unit := splitUnits(hn)

		cols[i] = &colAcc{name: clean, unit: unit, origUnit: unit, min: math.Inf(1), max: math.Inf(-1), cats: make(map[string]int)}

		gbIndex[strings.ToLower(clean)] = i
	}

	rep := &Report{Name: filepath.Base(path)}
	if droppedCols > 0 {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("dropped %d column(s) due to MaxColumns limit", droppedCols))
	}
	maxRows := opt.MaxRows
	if maxRows <= 0 {
		maxRows = math.MaxInt
	}
	sampleRows := opt.SampleRows
	if sampleRows < 0 {
		sampleRows = 5
	}
	var numericVals [][]float64

	// Exact pairwise correlation accumulators with missingness handling.
	type pairAcc struct {
		n     float64
		sumX  float64
		sumY  float64
		sumXX float64
		sumYY float64
		sumXY float64
	}
	pair := make(map[int]*pairAcc) // key = i*ncol + j with i>j

	// Group-by accumulators
	type gAcc struct {
		size int
		// per numeric column
		sum map[int]float64
		cnt map[int]int
		min map[int]float64
		max map[int]float64
	}
	groups := map[string]*gAcc{}
	// Per-group correlation accumulators
	gPairs := map[string]map[int]*pairAcc{}

	for {
		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read row %d: %w", rep.Rows+1, err)
		}
		rep.Rows++
		// Normalize length
		if len(rec) < ncol {
			// pad
			tmp := make([]string, ncol)
			copy(tmp, rec)
			rec = tmp
		}

		if rep.Processed >= maxRows {
			continue
		}
		rep.Processed++

		if opt.MaxColumns > 0 && len(rec) > opt.MaxColumns {
			rec = rec[:opt.MaxColumns]
		}

		if len(rep.Samples) < sampleRows {
			rowCopy := make([]string, ncol)
			copy(rowCopy, rec)
			rep.Samples = append(rep.Samples, rowCopy)
		}
		// Optional: compute group key once per row
		var gkey string
		if len(opt.GroupBy) > 0 {
			var parts []string
			for _, name := range opt.GroupBy {
				idx, ok := gbIndex[strings.ToLower(strings.TrimSpace(name))]
				if !ok || idx >= len(rec) {
					continue
				}
				val := strings.TrimSpace(rec[idx])
				parts = append(parts, fmt.Sprintf("%s=%s", cols[idx].name, safeVal(val)))
			}
			if len(parts) > 0 {
				gkey = strings.Join(parts, " | ")
			}
		}

		// Track numeric values for this row to feed pairwise accumulators once per row
		rowNums := make(map[int]float64)
		// For outlier detection, store numeric stream per column
		if numericVals == nil {
			numericVals = make([][]float64, ncol)
		}
		for j := 0; j < ncol; j++ {
			v := strings.TrimSpace(rec[j])
			if v == "" {
				cols[j].miss++
				continue
			}
			c := cols[j]
			c.nonNil++
			// Try numeric first
			if strings.Contains(v, "%") && c.unit == "" {
				c.unit = "%"

				if c.origUnit == "" {
					c.origUnit = "%"
				}
			}
			if x, ok := parseNumeric(v, c.unit, opt); ok {
				if opt.UnitNormalize && c.origUnit != "" {
					if nx, nu, okc := normalizeUnit(x, c.origUnit, opt); okc {

						x = nx
						c.unit = nu
					}
				}
				c.numCnt++
				// Welford update
				c.n++
				if x < c.min {
					c.min = x
				}
				if x > c.max {
					c.max = x
				}
				delta := x - c.mean
				c.mean += delta / float64(c.n)
				c.m2 += delta * (x - c.mean)
				if opt.Correlations {
					rowNums[j] = x
				}
				// collect for outlier detection
				numericVals[j] = append(numericVals[j], x)
				// Group-by accumulate
				if gkey != "" {
					ga := groups[gkey]
					if ga == nil {
						ga = &gAcc{sum: map[int]float64{}, cnt: map[int]int{}, min: map[int]float64{}, max: map[int]float64{}}
						groups[gkey] = ga
					}
					ga.sum[j] += x
					ga.cnt[j]++
					if _, ok := ga.min[j]; !ok || x < ga.min[j] {
						ga.min[j] = x
					}
					if _, ok := ga.max[j]; !ok || x > ga.max[j] {
						ga.max[j] = x
					}
				}
				continue
			}
			// Try datetime
			if _, ok := parseTimeMaybe(v); ok {
				c.dtCnt++
				continue
			}
			// Text/categorical
			c.txtCnt++
			if len(c.cats) <= 10000 { // guard memory
				if len(v) <= 64 {
					c.cats[v]++
				} // treat short tokens as categories
			}
			if len(c.exText) < 3 {
				c.exText = append(c.exText, v)
			}
		}
		// Increment group size once per row (if applicable)
		if gkey != "" {
			ga := groups[gkey]
			if ga == nil {
				ga = &gAcc{sum: map[int]float64{}, cnt: map[int]int{}, min: map[int]float64{}, max: map[int]float64{}}
				groups[gkey] = ga
			}
			ga.size++
		}
		// Update exact pairwise correlation accumulators for this row
		if opt.Correlations && len(rowNums) >= 2 {
			// iterate j>k pairs from keys present in rowNums
			// collect keys for stable iteration
			idxs := make([]int, 0, len(rowNums))
			for j := range rowNums {
				idxs = append(idxs, j)
			}
			sort.Ints(idxs)
			for a := 1; a < len(idxs); a++ {
				j := idxs[a]
				x := rowNums[j]
				for b := 0; b < a; b++ {
					k := idxs[b]
					y := rowNums[k]
					key := j*ncol + k
					pa := pair[key]
					if pa == nil {
						pa = &pairAcc{}
						pair[key] = pa
					}
					pa.n += 1
					pa.sumX += x
					pa.sumY += y
					pa.sumXX += x * x
					pa.sumYY += y * y
					pa.sumXY += x * y
				}
			}
		}
		// Per-group pairwise if requested
		if opt.CorrPerGroup && gkey != "" && len(rowNums) >= 2 {
			idxs := make([]int, 0, len(rowNums))
			for j := range rowNums {
				idxs = append(idxs, j)
			}
			sort.Ints(idxs)
			gp := gPairs[gkey]
			if gp == nil {
				gp = map[int]*pairAcc{}
				gPairs[gkey] = gp
			}
			for a := 1; a < len(idxs); a++ {
				j := idxs[a]
				x := rowNums[j]
				for b := 0; b < a; b++ {
					k := idxs[b]
					y := rowNums[k]
					key := j*ncol + k
					pa := gp[key]
					if pa == nil {
						pa = &pairAcc{}
						gp[key] = pa
					}
					pa.n += 1
					pa.sumX += x
					pa.sumY += y
					pa.sumXX += x * x
					pa.sumYY += y * y
					pa.sumXY += x * y
				}
			}
		}
	}

	// Build summaries
	rep.Cols = make([]ColumnSummary, 0, ncol)
	numCols := []int{}
	for idx, c := range cols {
		if c == nil {
			continue
		}
		s := ColumnSummary{Name: c.name, Unit: c.unit, NonNull: c.nonNil, Missing: c.miss}
		// Decide kind by predominant parsed type
		kind := "unknown"
		if c.numCnt >= c.dtCnt && c.numCnt >= c.txtCnt && c.numCnt > 0 {
			kind = "numeric"
			s.Min = c.min
			s.Max = c.max
			s.Mean = c.mean
			if c.n > 1 {
				s.Std = math.Sqrt(c.m2 / float64(c.n-1))
			}
			numCols = append(numCols, idx)
			if opt.Outliers && len(numericVals[idx]) >= 8 {
				median, mad := medianMAD(numericVals[idx])
				thr := opt.OutlierThreshold
				if thr <= 0 {
					thr = 3.5
				}
				var cnt int
				maxAbsZ := 0.0
				if mad > 0 {
					for _, v := range numericVals[idx] {
						z := 0.6745 * (v - median) / mad
						az := math.Abs(z)
						if az > thr {
							cnt++
						}
						if az > maxAbsZ {
							maxAbsZ = az
						}
					}
				}
				s.OutliersCount = cnt
				s.OutliersMaxAbsZ = maxAbsZ
				s.OutlierThreshold = thr
				// FREE MEMORY: Clear the array after outlier computation
				numericVals[idx] = nil
			}
		} else if c.dtCnt >= c.txtCnt && c.dtCnt > 0 {
			kind = "datetime"
		} else if len(c.cats) > 0 {
			kind = "categorical"
			// top values
			tops := make([]CategoryCount, 0, len(c.cats))
			for k, v := range c.cats {
				tops = append(tops, CategoryCount{Value: k, Count: v})
			}
			sort.Slice(tops, func(i, j int) bool {
				if tops[i].Count == tops[j].Count {
					return tops[i].Value < tops[j].Value
				}
				return tops[i].Count > tops[j].Count
			})
			if len(tops) > 8 {
				tops = tops[:8]
			}
			s.TopValues = tops
			s.Unique = len(c.cats)
		} else if c.txtCnt > 0 {
			kind = "text"
			s.ExampleTexts = c.exText
		}
		s.Kind = kind
		rep.Cols = append(rep.Cols, s)
	}

	if rep.Processed < rep.Rows {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("processed only %d/%d rows due to MaxRows", rep.Processed, rep.Rows))
	}

	// Build group-by results
	if len(groups) > 0 {
		out := make([]GroupResult, 0, len(groups))
		for k, ga := range groups {
			gr := GroupResult{Key: k, Size: ga.size, Metrics: map[string]NumSummary{}}
			for _, idx := range numCols {
				if ga.cnt[idx] == 0 {
					continue
				}
				name := cols[idx].name
				gr.Metrics[name] = NumSummary{Count: ga.cnt[idx], Min: ga.min[idx], Max: ga.max[idx], Mean: ga.sum[idx] / float64(ga.cnt[idx])}
			}
			// Per-group correlations summary
			if opt.CorrPerGroup {
				if gp := gPairs[k]; gp != nil {
					// reconstruct pairs r
					var pairs []PairCorr
					for key, pa := range gp {
						if pa == nil || pa.n < 2 {
							continue
						}
						// decode indices
						j := key / ncol
						k2 := key % ncol
						denom := math.Sqrt((pa.n*pa.sumXX - pa.sumX*pa.sumX) * (pa.n*pa.sumYY - pa.sumY*pa.sumY))
						if denom == 0 {
							continue
						}
						r := (pa.n*pa.sumXY - pa.sumX*pa.sumY) / denom
						if r > 1 {
							r = 1
						} else if r < -1 {
							r = -1
						}
						if math.IsNaN(r) || math.IsInf(r, 0) {
							continue
						}
						pairs = append(pairs, PairCorr{A: cols[k2].name, B: cols[j].name, R: r})
					}
					sort.Slice(pairs, func(i, j int) bool {
						ai, aj := math.Abs(pairs[i].R), math.Abs(pairs[j].R)
						if ai == aj {
							return pairs[i].A+pairs[i].B < pairs[j].A+pairs[j].B
						}
						return ai > aj
					})
					if len(pairs) > 10 {
						pairs = pairs[:10]
					}
					gr.CorrPairs = pairs
				}
			}
			out = append(out, gr)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Size == out[j].Size {
				return out[i].Key < out[j].Key
			}
			return out[i].Size > out[j].Size
		})
		if len(out) > 20 {
			out = out[:20]
		}
		rep.Groups = out
	}

	// Build correlation matrix (global, across numeric columns only)
	if opt.Correlations && len(numCols) >= 2 {
		colsNames := make([]string, len(numCols))
		for i, idx := range numCols {
			colsNames[i] = cols[idx].name
		}
		n := len(numCols)
		mat := make([][]float64, n)
		for i := range mat {
			mat[i] = make([]float64, n)
		}
		for a := 0; a < n; a++ {
			ia := numCols[a]
			for b := 0; b < n; b++ {
				if a == b {
					mat[a][b] = 1
					continue
				}
				ib := numCols[b]
				key := max(ia, ib)*ncol + min(ia, ib)
				if pa := pair[key]; pa != nil && pa.n >= 2 {
					denom := math.Sqrt((pa.n*pa.sumXX - pa.sumX*pa.sumX) * (pa.n*pa.sumYY - pa.sumY*pa.sumY))
					var r float64
					if denom != 0 {
						r = (pa.n*pa.sumXY - pa.sumX*pa.sumY) / denom
					}
					if r > 1 {
						r = 1
					} else if r < -1 {
						r = -1
					}
					if math.IsNaN(r) || math.IsInf(r, 0) {
						r = 0
					}
					mat[a][b] = r
				} else {
					mat[a][b] = 0
				}
			}
		}
		rep.Corr = &CorrMatrix{Columns: colsNames, Values: mat}
	}
	// Per-group correlations computed during the main pass and emitted in GroupResult.CorrPairs
	return rep, nil
}


func sniffDelimiter(path string) rune {
	name := strings.ToLower(path)
	if strings.HasSuffix(name, ".tsv") {
		return '\t'
	}
	// Default to comma; using filename heuristic only to avoid reading twice in restricted env.
	return ','

}

func parseTimeMaybe(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339, "2006-01-02", "2006/01/02", "02/01/2006", "01/02/2006",
		"2006-01-02 15:04", "2006-01-02 15:04:05", "1/2/2006 15:04", "1/2/2006 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseNumeric(s string, unit string, opt Options) (float64, bool) {
	raw := strings.TrimSpace(s)
	if strings.Contains(raw, "%") {
		raw = strings.ReplaceAll(raw, "%", "")
	}
	// Normalize spaces
	raw = strings.ReplaceAll(raw, "\u00A0", " ")
	raw = strings.TrimSpace(raw)
	// Decide decimal separator
	dec := opt.DecimalSeparator
	thou := opt.ThousandsSeparator
	if dec == 0 {
		// auto detect
		cpos := strings.LastIndex(raw, ",")
		dpos := strings.LastIndex(raw, ".")
		if cpos >= 0 && dpos >= 0 {
			if cpos > dpos {
				dec = ','
				thou = '.'
			} else {
				dec = '.'
				thou = ','
			}
		} else if cpos >= 0 {
			dec = ','
		} else {
			dec = '.'
		}
	}
	// Remove thousands separators (common: ',', '.', space) if they differ from decimal
	if thou == 0 {
		for _, sep := range []rune{',', '.', ' '} {
			if sep != dec {
				raw = strings.ReplaceAll(raw, string(sep), "")
			}
		}
	} else if thou != dec {
		raw = strings.ReplaceAll(raw, string(thou), "")
	}
	// Replace decimal with '.'
	if dec != '.' {
		raw = strings.ReplaceAll(raw, string(dec), ".")
	}
	// Scientific notation is now compatible: e/E kept as-is
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func normalizeUnit(x float64, unit string, opt Options) (float64, string, bool) {
	if opt.UnitTargets == nil {
		return x, unit, false
	}
	target, ok := opt.UnitTargets[unit]
	if !ok {
		return x, unit, false
	}
	switch unit + ">" + target {
	case "g/L>mg/L":
		return x * 1000, target, true
	case "ug/L>mg/L":
		return x / 1000, target, true
	case "°F>°C":
		return (x - 32) * 5.0 / 9.0, target, true
	default:
		return x, unit, false
	}
}

// Markdown renders a compact report suitable for prompts or standalone docs.
func (r *Report) Markdown() string {
	var b strings.Builder
	b.WriteString("[DATASET SUMMARY]\n")
	if r.Name != "" {
		b.WriteString(fmt.Sprintf("File: %s\n", r.Name))
	}
	if r.Rows > 0 {
		if r.Processed > 0 && r.Processed < r.Rows {
			b.WriteString(fmt.Sprintf("Rows: ~%d (processed %d)\n", r.Rows, r.Processed))
		} else {
			b.WriteString(fmt.Sprintf("Rows: %d\n", r.Rows))
		}
	}
	b.WriteString(fmt.Sprintf("Columns: %d\n\n", len(r.Cols)))

	b.WriteString("[SCHEMA]\n")
	for _, c := range r.Cols {
		nn := c.NonNull
		total := c.NonNull + c.Missing
		missPct := 0.0
		if total > 0 {
			missPct = float64(c.Missing) * 100.0 / float64(total)
		}
		name := safeName(c.Name)
		if c.Unit != "" {
			name = fmt.Sprintf("%s [%s]", name, c.Unit)
		}
		b.WriteString(fmt.Sprintf("- %s: %s (non-null %d, missing %.1f%%)", name, c.Kind, nn, missPct))
		switch c.Kind {
		case "numeric":
			b.WriteString(fmt.Sprintf(" — min %.4g, max %.4g, mean %.4g, std %.4g", c.Min, c.Max, c.Mean, c.Std))
			if c.OutlierThreshold > 0 {
				b.WriteString(fmt.Sprintf("; outliers: %d above |z|>%.1f", c.OutliersCount, c.OutlierThreshold))
				if c.OutliersMaxAbsZ > 0 {
					b.WriteString(fmt.Sprintf(" (max |z|≈%.2f)", c.OutliersMaxAbsZ))
				}
			}
		case "categorical":
			if len(c.TopValues) > 0 {
				b.WriteString(" — top: ")
				for i, kv := range c.TopValues {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(fmt.Sprintf("%s(%d)", safeVal(kv.Value), kv.Count))
				}
				if c.Unique > len(c.TopValues) {
					b.WriteString(fmt.Sprintf("; unique=%d", c.Unique))
				}
			}
		case "text":
			if len(c.ExampleTexts) > 0 {
				b.WriteString(" — e.g., ")
				for i, ex := range c.ExampleTexts {
					if i > 0 {
						b.WriteString(" | ")
					}
					b.WriteString(safeVal(ex))
				}
			}
		}
		b.WriteString("\n")
	}
	if len(r.Groups) > 0 {
		b.WriteString("\n[GROUP-BY SUMMARY]\n")
		for _, g := range r.Groups {
			b.WriteString(fmt.Sprintf("- %s (n=%d)\n", g.Key, g.Size))
			// print up to 6 metrics
			keys := make([]string, 0, len(g.Metrics))
			for k := range g.Metrics {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			maxk := 6
			if len(keys) < maxk {
				maxk = len(keys)
			}
			for i := 0; i < maxk; i++ {
				m := g.Metrics[keys[i]]
				b.WriteString(fmt.Sprintf("  • %s: mean %.4g (min %.4g, max %.4g)\n", keys[i], m.Mean, m.Min, m.Max))
			}
		}
	}
	// Per-group correlations section if present
	hasGCorr := false
	for _, g := range r.Groups {
		if len(g.CorrPairs) > 0 {
			hasGCorr = true
			break
		}
	}
	if hasGCorr {
		b.WriteString("\n[PER-GROUP CORRELATIONS]\n")
		for _, g := range r.Groups {
			if len(g.CorrPairs) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("- %s:\n", g.Key))
			lim := 8
			if len(g.CorrPairs) < lim {
				lim = len(g.CorrPairs)
			}
			for i := 0; i < lim; i++ {
				p := g.CorrPairs[i]
				b.WriteString(fmt.Sprintf("  • %s ~ %s: r=%.3f\n", p.A, p.B, p.R))
			}
		}
	}
	if r.Corr != nil && len(r.Corr.Columns) >= 2 {
		b.WriteString("\n[CORRELATIONS]\n")
		// list top pairs by |r|
		type pr struct {
			A, B string
			R    float64
		}
		var pairs []pr
		n := len(r.Corr.Columns)
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				pairs = append(pairs, pr{A: r.Corr.Columns[i], B: r.Corr.Columns[j], R: r.Corr.Values[i][j]})
			}
		}
		sort.Slice(pairs, func(i, j int) bool {
			ai := math.Abs(pairs[i].R)
			aj := math.Abs(pairs[j].R)
			if ai == aj {
				return pairs[i].A+pairs[i].B < pairs[j].A+pairs[j].B
			}
			return ai > aj
		})
		maxp := 10
		if len(pairs) < maxp {
			maxp = len(pairs)
		}
		for i := 0; i < maxp; i++ {
			b.WriteString(fmt.Sprintf("- %s ~ %s: r=%.3f\n", pairs[i].A, pairs[i].B, pairs[i].R))
		}
	}
	if len(r.Samples) > 0 {
		b.WriteString("\n[HEAD AND SAMPLE ROWS]\n")
		// Header
		b.WriteString("| ")
		for i, c := range r.Cols {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(safeName(c.Name))
		}
		b.WriteString(" |\n")
		b.WriteString("| ")
		for i := range r.Cols {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString("---")
		}
		b.WriteString(" |\n")
		for _, row := range r.Samples {
			b.WriteString("| ")
			for i := range r.Cols {
				if i > 0 {
					b.WriteString(" | ")
				}
				val := ""
				if i < len(row) {
					val = row[i]
				}
				if len(val) > 80 {
					val = val[:77] + "..."
				}
				b.WriteString(safeVal(val))
			}
			b.WriteString(" |\n")
		}
	}
	if len(r.Warnings) > 0 {
		b.WriteString("\n[NOTES]\n")
		for _, w := range r.Warnings {
			b.WriteString("- ")
			b.WriteString(w)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func safeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(unnamed)"
	}
	return s
}
func safeVal(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "|", "/") }

var unitPatterns = []struct {
	re   *regexp.Regexp
	pick int
}{
	{regexp.MustCompile(`^(.*)\s*\(([^)]+)\)\s*$`), 2},  // e.g., Alpha (%)
	{regexp.MustCompile(`^(.*)\s*\[([^\]]+)\]\s*$`), 2}, // e.g., Mass [mg/L]
	{regexp.MustCompile(`^(.*?)[_\s-]+(mg/L|g/L|ug/L|°[CF]|Brix|%|ppm|ppb)$`), 2},
}

func splitUnits(name string) (clean string, unit string) {
	s := strings.TrimSpace(name)
	for _, p := range unitPatterns {
		if m := p.re.FindStringSubmatch(s); len(m) >= 3 {
			base := strings.TrimSpace(m[1])
			u := strings.TrimSpace(m[p.pick])
			if base != "" && u != "" {
				return base, u
			}
		}
	}
	return s, ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// medianMAD computes median and MAD (median absolute deviation) of values.
func medianMAD(vals []float64) (median, mad float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	cp := make([]float64, len(vals))
	copy(cp, vals)
	sort.Float64s(cp)
	median = quantile(cp, 0.5)
	dev := make([]float64, len(cp))
	for i, v := range cp {
		d := v - median
		if d < 0 {
			d = -d
		}
		dev[i] = d
	}
	sort.Float64s(dev)
	mad = quantile(dev, 0.5)
	return
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	w := pos - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}
