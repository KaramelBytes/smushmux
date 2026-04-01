package analysis

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AnalyzeXLSX parses a .xlsx file, extracts rows from the selected sheet, and computes a Report.
// If sheetName is empty and sheetIndex <= 0, it defaults to the first sheet.
// sheetIndex is 1-based (Sheet1 == 1).
func AnalyzeXLSX(path string, opt Options, sheetName string, sheetIndex int) (*Report, error) {
	// Load ZIP
	b, err := osReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	// Gather key files
	workbookXML := readZipFile(zr, "xl/workbook.xml")
	relsXML := readZipFile(zr, "xl/_rels/workbook.xml.rels")
	sharedXML := readZipFile(zr, "xl/sharedStrings.xml")
	sheets := parseWorkbook(workbookXML)
	rels := parseRelationships(relsXML)
	// Resolve target sheet path
	target := ""
	if sheetName != "" {
		for _, s := range sheets {
			if strings.EqualFold(s.Name, sheetName) {
				if rel, ok := rels[s.RID]; ok {
					target = normalizeRelPath(rel)
				}
				break
			}
		}
	}
	if sheetName != "" && target == "" {
		// Sheet name was requested but not found
		availableSheets := make([]string, len(sheets))
		for i, s := range sheets {
			availableSheets[i] = s.Name
		}

		return nil, fmt.Errorf("sheet '%s' not found in workbook '%s'.\nAvailable sheets: %s",
			sheetName, filepath.Base(path), strings.Join(availableSheets, ", "))
	}

	if target == "" {
		// fallback by index (1-based)
		idx := sheetIndex
		if idx <= 0 {
			idx = 1
		}
		// find sheet with sheetId == idx, otherwise guess by worksheets/sheetN.xml
		var rid string
		for _, s := range sheets {
			if s.SheetID == idx {
				rid = s.RID
				break
			}
		}
		if rid != "" {
			if rel, ok := rels[rid]; ok {
				target = normalizeRelPath(rel)
			}
		}
		if target == "" {
			target = filepath.Join("xl", "worksheets", fmt.Sprintf("sheet%d.xml", idx))
		}
	}
	sheetXML := readZipFile(zr, target)
	shared := parseSharedStrings(sharedXML)
	// Iterate rows
	rr := newSheetRowReader(sheetXML, shared)
	header, ok := rr.Next()
	if !ok || len(header) == 0 {
		rep := &Report{Name: filepath.Base(path)}
		return rep, nil
	}

	var droppedCols int
	if opt.MaxColumns > 0 && len(header) > opt.MaxColumns {
		droppedCols = len(header) - opt.MaxColumns
		header = header[:opt.MaxColumns]
	}

	// We will feed records to the same aggregator logic as AnalyzeCSV by reusing the internal core.
	// To avoid heavy refactor, duplicate a small segment of the AnalyzeCSV accumulation code here.
	// Build column accumulators
	type colAcc struct {
		name     string
		unit     string
		origUnit string
		nonNil   int
		miss     int
		n        int
		mean     float64
		m2       float64
		min      float64
		max      float64
		numCnt   int
		dtCnt    int
		txtCnt   int
		cats     map[string]int
		exText   []string
	}
	ncol := len(header)
	cols := make([]*colAcc, ncol)
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
		maxRows = int(^uint(0) >> 1)
	}
	sampleRows := opt.SampleRows
	if sampleRows < 0 {
		sampleRows = 5
	}
	var numericVals [][]float64

	// Exact pairwise correlations with missingness
	type pairAcc struct {
		n     float64
		sumX  float64
		sumY  float64
		sumXX float64
		sumYY float64
		sumXY float64
	}
	pair := make(map[int]*pairAcc)
	type gAcc struct {
		size int
		sum  map[int]float64
		min  map[int]float64
		max  map[int]float64
		cnt  map[int]int
	}
	groups := map[string]*gAcc{}
	gPairs := map[string]map[int]*pairAcc{}

	// already consumed header
	for {
		row, ok := rr.Next()
		if !ok {
			break
		}
		rep.Rows++
		if len(row) < ncol {
			tmp := make([]string, ncol)
			copy(tmp, row)
			row = tmp
		} else if opt.MaxColumns > 0 && len(row) > opt.MaxColumns {
			row = row[:opt.MaxColumns]
		}
		if rep.Processed >= maxRows {
			continue
		}
		rep.Processed++
		if len(rep.Samples) < sampleRows {
			cp := make([]string, ncol)
			copy(cp, row)
			rep.Samples = append(rep.Samples, cp)
		}
		// group key
		var gkey string
		if len(opt.GroupBy) > 0 {
			var parts []string
			for _, name := range opt.GroupBy {
				idx, ok := gbIndex[strings.ToLower(strings.TrimSpace(name))]
				if !ok || idx >= len(row) {
					continue
				}
				parts = append(parts, fmt.Sprintf("%s=%s", cols[idx].name, safeVal(strings.TrimSpace(row[idx]))))
			}
			if len(parts) > 0 {
				gkey = strings.Join(parts, " | ")
			}
		}
		// per-row numeric cache for pairwise updates
		rowNums := make(map[int]float64)
		for j := 0; j < ncol; j++ {
			v := strings.TrimSpace(row[j])
			if v == "" {
				cols[j].miss++
				continue
			}
			c := cols[j]
			c.nonNil++
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
				if numericVals == nil {
					numericVals = make([][]float64, ncol)
				}
				numericVals[j] = append(numericVals[j], x)
				if gkey != "" {
					ga := groups[gkey]
					if ga == nil {
						ga = &gAcc{sum: map[int]float64{}, min: map[int]float64{}, max: map[int]float64{}, cnt: map[int]int{}}
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
			if _, ok := parseTimeMaybe(v); ok {
				c.dtCnt++
				continue
			}
			c.txtCnt++
			if len(c.cats) <= 10000 {
				if len(v) <= 64 {
					c.cats[v]++
				}
			}
			if len(c.exText) < 3 {
				c.exText = append(c.exText, v)
			}
		}
		if gkey != "" {
			ga := groups[gkey]
			if ga == nil {
				ga = &gAcc{sum: map[int]float64{}, min: map[int]float64{}, max: map[int]float64{}, cnt: map[int]int{}}
				groups[gkey] = ga
			}
			ga.size++
		}
		if opt.Correlations && len(rowNums) >= 2 {
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

	// finalize columns
	numCols := []int{}
	for i, c := range cols {
		s := ColumnSummary{Name: c.name, Unit: c.unit, NonNull: c.nonNil, Missing: c.miss}
		kind := "unknown"
		if c.numCnt >= c.dtCnt && c.numCnt >= c.txtCnt && c.numCnt > 0 {
			kind = "numeric"
			s.Min = c.min
			s.Max = c.max
			s.Mean = c.mean
			if c.n > 1 {
				s.Std = math.Sqrt(c.m2 / float64(c.n-1))
			}
			numCols = append(numCols, i)
			if opt.Outliers && len(numericVals[i]) >= 8 {
				median, mad := medianMAD(numericVals[i])
				thr := opt.OutlierThreshold
				if thr <= 0 {
					thr = 3.5
				}
				var cnt int
				maxAbsZ := 0.0
				if mad > 0 {
					for _, v := range numericVals[i] {
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
				numericVals[i] = nil
			}
		} else if c.dtCnt >= c.txtCnt && c.dtCnt > 0 {
			kind = "datetime"
		} else if len(c.cats) > 0 {
			kind = "categorical"
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

	// groups
	if len(groups) > 0 {
		outs := make([]GroupResult, 0, len(groups))
		for k, ga := range groups {
			gr := GroupResult{Key: k, Size: ga.size, Metrics: map[string]NumSummary{}}
			for _, idx := range numCols {
				if ga.cnt[idx] == 0 {
					continue
				}
				gr.Metrics[cols[idx].name] = NumSummary{Count: ga.cnt[idx], Min: ga.min[idx], Max: ga.max[idx], Mean: ga.sum[idx] / float64(ga.cnt[idx])}
			}
			if opt.CorrPerGroup {
				if gp := gPairs[k]; gp != nil {
					var pairs []PairCorr
					for key, pa := range gp {
						if pa == nil || pa.n < 2 {
							continue
						}
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
			outs = append(outs, gr)
		}
		sort.Slice(outs, func(i, j int) bool {
			if outs[i].Size == outs[j].Size {
				return outs[i].Key < outs[j].Key
			}
			return outs[i].Size > outs[j].Size
		})
		if len(outs) > 20 {
			outs = outs[:20]
		}
		rep.Groups = outs
	}
	if rep.Processed < rep.Rows {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("processed only %d/%d rows due to MaxRows", rep.Processed, rep.Rows))
	}

	if opt.Correlations && len(numCols) >= 2 {
		names := make([]string, len(numCols))
		for i, idx := range numCols {
			names[i] = cols[idx].name
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
				pa := pair[key]
				if pa == nil || pa.n < 2 {
					mat[a][b] = 0
					continue
				}
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
			}
		}
		rep.Corr = &CorrMatrix{Columns: names, Values: mat}
	}
	return rep, nil
}

// Minimal helpers to avoid importing math/os in this file twice when used by the sandbox patcher.
func osReadFile(p string) ([]byte, error) { return os.ReadFile(p) }

// parseWorkbook extracts sheet entries with names and relationship ids.
func parseWorkbook(data []byte) []wbSheet {
	if len(data) == 0 {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var sheets []wbSheet
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return sheets
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "sheet" {
				var s wbSheet
				for _, a := range se.Attr {
					switch a.Name.Local {
					case "name":
						s.Name = a.Value
					case "sheetId":
						s.SheetID = atoiSafe(a.Value)
					case "id":
						s.RID = a.Value // in r: namespace
					}
				}
				sheets = append(sheets, s)
			}
		}
	}
	return sheets
}

type wbSheet struct {
	Name    string
	SheetID int
	RID     string
}

func parseRelationships(data []byte) map[string]string {
	// returns map[r:id]Target
	out := map[string]string{}
	if len(data) == 0 {
		return out
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return out
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "Relationship" {
				var id, target string
				for _, a := range se.Attr {
					switch a.Name.Local {
					case "Id":
						id = a.Value
					case "Target":
						target = a.Value
					}
				}
				if id != "" && target != "" {
					out[id] = target
				}
			}
		}
	}
	return out
}

func readZipFile(zr *zip.Reader, name string) []byte {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return b
		}
	}
	return nil
}

// shared strings
func parseSharedStrings(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out []string
	var buf strings.Builder
	var inT bool
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return out
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "si" {
				buf.Reset()
			}
			if se.Name.Local == "t" {
				inT = true
			}
		case xml.EndElement:
			if se.Name.Local == "t" {
				inT = false
			}
			if se.Name.Local == "si" {
				out = append(out, buf.String())
				buf.Reset()
			}
		case xml.CharData:
			if inT {
				buf.Write([]byte(se))
			}
		}
	}
	return out
}

// sheet row reader
type sheetRowReader struct {
	dec    *xml.Decoder
	shared []string
	inRow  bool
	curRow []string
	maxCol int
}

func newSheetRowReader(data []byte, shared []string) *sheetRowReader {
	return &sheetRowReader{dec: xml.NewDecoder(bytes.NewReader(data)), shared: shared}
}

func (r *sheetRowReader) Next() ([]string, bool) {
	for {
		tok, err := r.dec.Token()
		if err != nil {
			return nil, false
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "row" {
				r.inRow = true
				r.curRow = nil
				r.maxCol = 0
			}
			if r.inRow && se.Name.Local == "c" {
				// cell: attributes r (A1), t (type)
				var rAttr, tAttr string
				for _, a := range se.Attr {
					switch a.Name.Local {
					case "r":
						rAttr = a.Value
					case "t":
						tAttr = a.Value
					}
				}
				colIdx := colIndexFromRef(rAttr)
				if colIdx+1 > r.maxCol {
					r.maxCol = colIdx + 1
				}
				val := r.readCellValue(tAttr)
				// ensure capacity
				if len(r.curRow) <= colIdx {
					tmp := make([]string, colIdx+1)
					copy(tmp, r.curRow)
					r.curRow = tmp
				}
				r.curRow[colIdx] = val
			}
		case xml.EndElement:
			if se.Name.Local == "row" {
				// normalize length
				if len(r.curRow) < r.maxCol {
					tmp := make([]string, r.maxCol)
					copy(tmp, r.curRow)
					r.curRow = tmp
				}
				r.inRow = false
				return r.curRow, true
			}
		}
	}
}

func (r *sheetRowReader) readCellValue(tAttr string) string {
	var val string
	// read until end of c; capture <v> or <is><t>
	for {
		tok, err := r.dec.Token()
		if err != nil {
			return val
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "v" || se.Name.Local == "t" {
				var sb strings.Builder
				for {
					tk, er := r.dec.Token()
					if er != nil {
						break
					}
					if ed, ok := tk.(xml.EndElement); ok && (ed.Name.Local == "v" || ed.Name.Local == "t") {
						break
					}
					if ch, ok := tk.(xml.CharData); ok {
						sb.Write([]byte(ch))
					}
				}
				val = sb.String()
			}
		case xml.EndElement:
			if se.Name.Local == "c" {
				if tAttr == "s" { // shared string
					idx := atoiSafe(val)
					if idx >= 0 && idx < len(r.shared) {
						return r.shared[idx]
					}
					return ""
				}
				return val
			}
		}
	}
}

// helpers for refs like "C12" -> 2 (0-based index)
func colIndexFromRef(ref string) int {
	i := 0
	for i < len(ref) {
		c := ref[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			i++
			continue
		}
		break
	}
	s := ref[:i]
	s = strings.ToUpper(s)
	idx := 0
	for j := 0; j < len(s); j++ {
		idx = idx*26 + int(s[j]-'A'+1)
	}
	return idx - 1
}

func atoiSafe(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// normalizeRelPath converts relationship Target paths to ZIP-compatible paths.
// Relationships may have leading slashes (e.g., "/xl/worksheets/sheet1.xml")
// but ZIP entries don't include the leading slash.
func normalizeRelPath(rel string) string {
	// Strip leading slash if present
	rel = strings.TrimPrefix(rel, "/")
	// If it already starts with "xl/", use as-is; otherwise prepend "xl/"
	if strings.HasPrefix(rel, "xl/") {
		return rel
	}
	return filepath.Join("xl", rel)
}
