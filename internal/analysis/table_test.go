package analysis

import (

	"encoding/base64"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var csvRows = []string{
	"Group;Concentration (g/L);Temp (°F);Score;LocaleNumber;Category;Note",
	"A;0,5;70;10,0;1.000,0;alpha;first",
	"A;0,6;71;11,0;1.100,0;alpha;second",
	"A;0,55;69;9,5;0.900,0;beta;third",
	"B;0,7;75;10,5;1.050,0;alpha;fourth",
	"B;0,65;74;9,8;0.980,0;beta;fifth",
	"B;0,68;73;10,2;1.020,0;alpha;sixth",
	"A;0,52;68;8,8;0.880,0;gamma;seventh",
	"B;0,75;76;9,7;0.970,0;beta;eighth",
	"A;3,0;95;50,0;5.000,0;alpha;ninth",
	"B;0,66;72;10,1;1.010,0;gamma;tenth",
}

var (
	processedConcentration = []float64{
		mgPerL(0.5), mgPerL(0.6), mgPerL(0.55), mgPerL(0.7), mgPerL(0.65), mgPerL(0.68), mgPerL(0.52), mgPerL(0.75), mgPerL(3.0),
	}
	processedTemp = []float64{
		toC(70), toC(71), toC(69), toC(75), toC(74), toC(73), toC(68), toC(76), toC(95),
	}
	processedScore  = []float64{10, 11, 9.5, 10.5, 9.8, 10.2, 8.8, 9.7, 50}
	processedLocale = []float64{1000, 1100, 900, 1050, 980, 1020, 880, 970, 5000}
)

const xlsxFixtureBase64 = `
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
`

func TestAnalyzeCSVAndMarkdown(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "metrics.csv")
	if err := os.WriteFile(csvPath, []byte(strings.Join(csvRows, "\n")), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	opt := DefaultOptions()
	opt.Delimiter = ';'
	opt.SampleRows = 3
	opt.MaxRows = 9
	opt.GroupBy = []string{"Group"}
	opt.Correlations = true
	opt.CorrPerGroup = true
	opt.Outliers = true
	opt.DecimalSeparator = ','
	opt.ThousandsSeparator = '.'

	rep, err := AnalyzeCSV(csvPath, opt)
	if err != nil {
		t.Fatalf("AnalyzeCSV: %v", err)
	}

	assertReport(t, rep, "metrics.csv")

	md := rep.Markdown()
	if !strings.Contains(md, "[DATASET SUMMARY]") {
		t.Fatalf("markdown missing summary: %s", md)
	}
	if !strings.Contains(md, "File: metrics.csv") {
		t.Fatalf("markdown missing file: %s", md)
	}
	if !strings.Contains(md, "Rows: ~10 (processed 9)") {
		t.Fatalf("markdown missing row note: %s", md)
	}
	if !strings.Contains(md, "Concentration [mg/L]: numeric") {
		t.Fatalf("markdown missing concentration schema: %s", md)
	}
	if !strings.Contains(md, "outliers: 1 above |z|>3.5") {
		t.Fatalf("markdown missing outlier info: %s", md)
	}
	if !strings.Contains(md, "[GROUP-BY SUMMARY]") || !strings.Contains(md, "Group=A (n=5)") {
		t.Fatalf("markdown missing group summary: %s", md)
	}
	if !strings.Contains(md, "[PER-GROUP CORRELATIONS]") || !strings.Contains(md, "Score ~ LocaleNumber") {
		t.Fatalf("markdown missing per-group correlations: %s", md)
	}
	if !strings.Contains(md, "[CORRELATIONS]") || !strings.Contains(md, "Score ~ LocaleNumber") {
		t.Fatalf("markdown missing global correlations: %s", md)
	}
	if !strings.Contains(md, "[NOTES]") || !strings.Contains(md, "processed only 9/10 rows due to MaxRows") {
		t.Fatalf("markdown missing notes: %s", md)
	}
}

func TestAnalyzeXLSXSheetSelectionAndMarkdown(t *testing.T) {
	t.Helper()
	opt := DefaultOptions()
	opt.SampleRows = 3
	opt.MaxRows = 9
	opt.GroupBy = []string{"Group"}
	opt.Correlations = true
	opt.CorrPerGroup = true
	opt.Outliers = true
	opt.DecimalSeparator = ','
	opt.ThousandsSeparator = '.'

	path := writeXLSXFixture(t)

	repByName, err := AnalyzeXLSX(path, opt, "Data", 0)
	if err != nil {
		t.Fatalf("AnalyzeXLSX name: %v", err)
	}
	assertReport(t, repByName, "analysis_dataset.xlsx")

	md := repByName.Markdown()
	if !strings.Contains(md, "File: analysis_dataset.xlsx") {
		t.Fatalf("xlsx markdown missing file: %s", md)
	}

	repByIndex, err := AnalyzeXLSX(path, opt, "", 2)
	if err != nil {
		t.Fatalf("AnalyzeXLSX index: %v", err)
	}
	assertReport(t, repByIndex, "analysis_dataset.xlsx")
}

func TestMaxColumnsCSV(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "wide.csv")
	// csvRows has 7 columns; limit to 3
	if err := os.WriteFile(csvPath, []byte(strings.Join(csvRows, "\n")), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	opt := DefaultOptions()
	opt.Delimiter = ';'
	opt.MaxColumns = 3

	rep, err := AnalyzeCSV(csvPath, opt)
	if err != nil {
		t.Fatalf("AnalyzeCSV: %v", err)
	}
	if len(rep.Cols) != 3 {
		t.Fatalf("cols = %d, want 3", len(rep.Cols))
	}
	var found bool
	for _, w := range rep.Warnings {
		if strings.Contains(w, "dropped 4 column(s) due to MaxColumns limit") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MaxColumns warning in Warnings, got %#v", rep.Warnings)
	}
}

func TestMaxColumnsXLSX(t *testing.T) {
	path := writeXLSXFixture(t)
	// The fixture Data sheet has 7 columns; limit to 3
	opt := DefaultOptions()
	opt.MaxColumns = 3

	rep, err := AnalyzeXLSX(path, opt, "Data", 0)
	if err != nil {
		t.Fatalf("AnalyzeXLSX: %v", err)
	}
	if len(rep.Cols) != 3 {
		t.Fatalf("cols = %d, want 3", len(rep.Cols))
	}
	var found bool
	for _, w := range rep.Warnings {
		if strings.Contains(w, "dropped") && strings.Contains(w, "MaxColumns limit") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MaxColumns warning in Warnings, got %#v", rep.Warnings)
	}
}

func writeXLSXFixture(t *testing.T) string {
	t.Helper()
	raw := strings.ReplaceAll(strings.TrimSpace(xlsxFixtureBase64), "\n", "")
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("decode xlsx fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "analysis_dataset.xlsx")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write xlsx fixture: %v", err)
	}
	return path
}

func assertReport(t *testing.T, rep *Report, expectName string) {
	t.Helper()
	if rep.Name != expectName {
		t.Fatalf("report name = %q, want %q", rep.Name, expectName)
	}
	if rep.Rows != 10 {
		t.Fatalf("rows = %d, want 10", rep.Rows)
	}
	if rep.Processed != 9 {
		t.Fatalf("processed = %d, want 9", rep.Processed)
	}
	if len(rep.Warnings) != 1 || rep.Warnings[0] != "processed only 9/10 rows due to MaxRows" {
		t.Fatalf("warnings = %#v", rep.Warnings)
	}
	if len(rep.Samples) != 3 {
		t.Fatalf("samples = %d, want 3", len(rep.Samples))
	}
	expectFirst := []string{"A", "0,5", "70", "10,0", "1.000,0", "alpha", "first"}
	if !equalStrings(rep.Samples[0], expectFirst) {
		t.Fatalf("first sample = %#v, want %#v", rep.Samples[0], expectFirst)
	}

	conc := columnByName(t, rep, "Concentration")
	if conc.Unit != "mg/L" {
		t.Fatalf("concentration unit = %q", conc.Unit)
	}
	checkStats(t, conc, processedConcentration)
	count, maxZ := robustOutlierStats(processedScore, 3.5)

	score := columnByName(t, rep, "Score")
	if score.Unit != "" {
		t.Fatalf("score unit = %q", score.Unit)
	}
	checkStats(t, score, processedScore)
	if score.OutliersCount != count {
		t.Fatalf("score outliers = %d, want %d", score.OutliersCount, count)
	}
	if !almostEqual(score.OutliersMaxAbsZ, maxZ, 1e-6) {
		t.Fatalf("score max |z| = %f, want %f", score.OutliersMaxAbsZ, maxZ)
	}
	if !almostEqual(score.OutlierThreshold, 3.5, 1e-9) {
		t.Fatalf("score threshold = %f", score.OutlierThreshold)
	}

	temp := columnByName(t, rep, "Temp")
	if temp.Unit != "°C" {
		t.Fatalf("temp unit = %q", temp.Unit)
	}
	checkStats(t, temp, processedTemp)

	locale := columnByName(t, rep, "LocaleNumber")
	checkStats(t, locale, processedLocale)

	cat := columnByName(t, rep, "Category")
	if cat.Kind != "categorical" {
		t.Fatalf("category kind = %q", cat.Kind)
	}
	if len(cat.TopValues) == 0 || cat.TopValues[0].Value != "alpha" || cat.TopValues[0].Count != 5 {
		t.Fatalf("category top = %#v", cat.TopValues)
	}

	if len(rep.Groups) != 2 {
		t.Fatalf("groups len = %d, want 2", len(rep.Groups))
	}
	groupA := rep.Groups[0]
	groupB := rep.Groups[1]
	if groupA.Key != "Group=A" || groupA.Size != 5 {
		t.Fatalf("group A = %#v", groupA)
	}
	if groupB.Key != "Group=B" || groupB.Size != 4 {
		t.Fatalf("group B = %#v", groupB)
	}

	scoreA := groupA.Metrics["Score"]
	scoreB := groupB.Metrics["Score"]
	checkNumSummary(t, scoreA, subset(processedScore, []int{0, 1, 2, 6, 8}))
	checkNumSummary(t, scoreB, subset(processedScore, []int{3, 4, 5, 7}))

	concA := groupA.Metrics["Concentration"]
	concB := groupB.Metrics["Concentration"]
	checkNumSummary(t, concA, subset(processedConcentration, []int{0, 1, 2, 6, 8}))
	checkNumSummary(t, concB, subset(processedConcentration, []int{3, 4, 5, 7}))

	expCorr := correlation(processedScore, processedLocale)
	if rep.Corr == nil {
		t.Fatalf("corr matrix nil")
	}
	if !equalStrings(rep.Corr.Columns, []string{"Concentration", "Temp", "Score", "LocaleNumber"}) {
		t.Fatalf("corr columns = %#v", rep.Corr.Columns)
	}
	idxScore := 2
	idxLocale := 3
	if !almostEqual(rep.Corr.Values[idxScore][idxLocale], expCorr, 1e-6) {
		t.Fatalf("global corr score-locale = %f, want %f", rep.Corr.Values[idxScore][idxLocale], expCorr)
	}

	corrA := correlation(subset(processedScore, []int{0, 1, 2, 6, 8}), subset(processedLocale, []int{0, 1, 2, 6, 8}))
	corrB := correlation(subset(processedScore, []int{3, 4, 5, 7}), subset(processedLocale, []int{3, 4, 5, 7}))

	if len(groupA.CorrPairs) == 0 || groupA.CorrPairs[0].A != "Score" || groupA.CorrPairs[0].B != "LocaleNumber" || !almostEqual(groupA.CorrPairs[0].R, corrA, 1e-6) {
		t.Fatalf("group A corr pairs = %#v", groupA.CorrPairs)
	}
	if len(groupB.CorrPairs) == 0 || groupB.CorrPairs[0].A != "Score" || groupB.CorrPairs[0].B != "LocaleNumber" || !almostEqual(groupB.CorrPairs[0].R, corrB, 1e-6) {
		t.Fatalf("group B corr pairs = %#v", groupB.CorrPairs)
	}
}

func columnByName(t *testing.T, rep *Report, name string) ColumnSummary {
	t.Helper()
	for _, c := range rep.Cols {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("column %q not found", name)
	return ColumnSummary{}
}

func checkStats(t *testing.T, col ColumnSummary, vals []float64) {
	t.Helper()
	if col.NonNull != len(vals) {
		t.Fatalf("non-null = %d, want %d", col.NonNull, len(vals))
	}
	if !almostEqual(col.Min, minFloat(vals), 1e-6) {
		t.Fatalf("min = %f, want %f", col.Min, minFloat(vals))
	}
	if !almostEqual(col.Max, maxFloat(vals), 1e-6) {
		t.Fatalf("max = %f, want %f", col.Max, maxFloat(vals))
	}
	if !almostEqual(col.Mean, mean(vals), 1e-6) {
		t.Fatalf("mean = %f, want %f", col.Mean, mean(vals))
	}
	if !almostEqual(col.Std, sampleStd(vals), 1e-6) {
		t.Fatalf("std = %f, want %f", col.Std, sampleStd(vals))
	}
}

func checkNumSummary(t *testing.T, s NumSummary, vals []float64) {
	t.Helper()
	if s.Count != len(vals) {
		t.Fatalf("summary count = %d, want %d", s.Count, len(vals))
	}
	if !almostEqual(s.Min, minFloat(vals), 1e-6) {
		t.Fatalf("summary min = %f, want %f", s.Min, minFloat(vals))
	}
	if !almostEqual(s.Max, maxFloat(vals), 1e-6) {
		t.Fatalf("summary max = %f, want %f", s.Max, maxFloat(vals))
	}
	if !almostEqual(s.Mean, mean(vals), 1e-6) {
		t.Fatalf("summary mean = %f, want %f", s.Mean, mean(vals))
	}
}

func robustOutlierStats(vals []float64, threshold float64) (count int, maxAbs float64) {
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	med := quantileValue(cp, 0.5)
	devs := make([]float64, len(cp))
	for i, v := range cp {
		d := math.Abs(v - med)
		devs[i] = d
	}
	sort.Float64s(devs)
	mad := quantileValue(devs, 0.5)
	if mad == 0 {
		return 0, 0
	}
	for _, v := range cp {
		z := 0.6745 * (v - med) / mad
		az := math.Abs(z)
		if az > threshold {
			count++
			if az > maxAbs {
				maxAbs = az
			}
		}
	}
	return
}

func quantileValue(sortedVals []float64, q float64) float64 {
	if len(sortedVals) == 0 {
		return 0
	}
	if q <= 0 {
		return sortedVals[0]
	}
	if q >= 1 {
		return sortedVals[len(sortedVals)-1]
	}
	pos := q * float64(len(sortedVals)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sortedVals[lo]
	}
	w := pos - float64(lo)
	return sortedVals[lo]*(1-w) + sortedVals[hi]*w
}

func subset(vals []float64, idxs []int) []float64 {
	out := make([]float64, len(idxs))
	for i, idx := range idxs {
		out[i] = vals[idx]
	}
	return out
}

func mean(vals []float64) float64 {
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func sampleStd(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	var sum float64
	for _, v := range vals {
		diff := v - m
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}

func minFloat(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxFloat(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func correlation(a, b []float64) float64 {
	if len(a) != len(b) {
		panic("length mismatch")
	}
	ma := mean(a)
	mb := mean(b)
	var num, da2, db2 float64
	for i := range a {
		da := a[i] - ma
		db := b[i] - mb
		num += da * db
		da2 += da * da
		db2 += db * db
	}
	if da2 == 0 || db2 == 0 {
		return 0
	}
	return num / math.Sqrt(da2*db2)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func mgPerL(v float64) float64 { return v * 1000 }
func toC(f float64) float64    { return (f - 32) * 5.0 / 9.0 }
