package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KaramelBytes/smushmux/internal/analysis"
	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/spf13/cobra"
)

var (
	abProject            string
	abDescription        string
	abDelimiter          string
	abSampleRows         int
	abMaxRows            int
	abGroupBy            []string
	abCorr               bool
	abCorrGroups         bool
	abDecimal            string
	abThousands          string
	abOutliers           bool
	abOutlierThr         float64
	abSheetName          string
	abSheetIndex         int
	abSampleRowsProject  int
	abQuiet              bool
)

var analyzeBatchCmd = &cobra.Command{
	Use:   "analyze-batch <files...>",
	Short: "Analyze multiple CSV/TSV/XLSX files with progress and optional project attachment",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var files []string
		seen := map[string]struct{}{}
		for _, arg := range args {
			matches, _ := filepath.Glob(arg)
			if len(matches) == 0 {
				// treat as literal path if exists
				if _, err := os.Stat(arg); err == nil {
					matches = []string{arg}
				}
			}
			for _, m := range matches {
				if _, ok := seen[m]; ok {
					continue
				}
				seen[m] = struct{}{}
				files = append(files, m)
			}
		}
		if len(files) == 0 {
			return fmt.Errorf("no input files matched")
		}
		sort.Strings(files)

		opt := analysis.DefaultOptions()
		if abSampleRows > 0 {
			opt.SampleRows = abSampleRows
		}
		if abMaxRows > 0 {
			opt.MaxRows = abMaxRows
		}
		if abDelimiter != "" {
			switch abDelimiter {
			case ",":
				opt.Delimiter = ','
			case "\t", "tab":
				opt.Delimiter = '\t'
			case ";":
				opt.Delimiter = ';'
			default:
				return fmt.Errorf("unsupported --delimiter: %s", abDelimiter)
			}
		}
		switch strings.ToLower(strings.TrimSpace(abDecimal)) {
		case ",", "comma":
			opt.DecimalSeparator = ','
		case ".", "dot":
			opt.DecimalSeparator = '.'
		case "":
		default:
			return fmt.Errorf("unsupported --decimal: %s (use '.'|'comma')", abDecimal)
		}
		switch strings.ToLower(strings.TrimSpace(abThousands)) {
		case ",":
			opt.ThousandsSeparator = ','
		case ".":
			opt.ThousandsSeparator = '.'
		case "space", " ":
			opt.ThousandsSeparator = ' '
		case "":
		default:
			return fmt.Errorf("unsupported --thousands: %s (use ','|'.'|'space')", abThousands)
		}
		opt.GroupBy = abGroupBy
		opt.Correlations = abCorr
		opt.CorrPerGroup = abCorrGroups
		if cmd.Flags().Changed("outliers") {
			opt.Outliers = abOutliers
		} else {
			opt.Outliers = true
		}
		if abOutlierThr > 0 {
			opt.OutlierThreshold = abOutlierThr
		}

		var p *project.Project
		if abProject != "" {
			projDir, err := resolveProjectDirByName(abProject)
			if err != nil {
				return err
			}
			pp, err := project.LoadProject(projDir)
			if err != nil {
				return err
			}
			p = pp
			if abSampleRowsProject >= 0 {
				opt.SampleRows = abSampleRowsProject
			}
		}

		total := len(files)
		for i, path := range files {
			if !abQuiet {
				fmt.Printf("[%d/%d] Processing %s...\n", i+1, total, filepath.Base(path))
			}
			lower := strings.ToLower(path)
			ext := strings.ToLower(filepath.Ext(lower))
			var md string
			var err error
			isTabular := false
			switch ext {
			case ".xlsx":
				isTabular = true
				rep, e := analysis.AnalyzeXLSX(path, opt, abSheetName, abSheetIndex)
				err = e
				if err == nil {
					md = rep.Markdown()
				}
			case ".csv", ".tsv":
				isTabular = true
				// If .tsv and delimiter not explicitly set, force tab
				if ext == ".tsv" && !cmd.Flags().Changed("delimiter") {
					opt.Delimiter = '\t'
				}
				rep, e := analysis.AnalyzeCSV(path, opt)
				err = e
				if err == nil {
					md = rep.Markdown()
				}
			}
			if !isTabular {
				// Non-tabular file: add as a regular document if project is provided; otherwise skip with a note.
				if p != nil {
					desc := abDescription
					if desc == "" {
						desc = "Added via analyze-batch (non-tabular)"
					}
					if err := p.AddDocument(path, desc); err != nil {
						// If duplicate or other error, warn and continue
						if !abQuiet {
							fmt.Printf("⚠ Skipped adding %s: %v\n", filepath.Base(path), err)
						}
					} else {
						if err := p.Save(); err != nil {
							return err
						}
						if !abQuiet {
							fmt.Printf("✓ Added document to project '%s' as %s\n", p.Name, filepath.Base(path))
						}
					}
					continue
				}
				if !abQuiet {
					fmt.Printf("⚠ Skipping non-tabular file without project: %s\n", filepath.Base(path))
				}
				continue
			}
			if err != nil {
				return err
			}

			written := false
			if p != nil {
				// project-level checks
				datasetCount := 0
				totalDatasetTokens := 0
				for _, doc := range p.Documents {
					desc := strings.ToLower(doc.Description)
					if strings.Contains(desc, "dataset") || strings.Contains(desc, "summary") ||
						strings.HasSuffix(doc.Name, ".summary.md") {
						datasetCount++
						totalDatasetTokens += doc.Tokens
					}
				}
				const maxDatasetSummaries = 20
				const maxDatasetTokens = 150000
				if datasetCount >= maxDatasetSummaries {
					return fmt.Errorf("project already has %d dataset summaries (limit: %d).\n  Consider: (1) Removing old summaries, (2) Using --retrieval mode, or (3) Creating a new project",
						datasetCount, maxDatasetSummaries)
				}
				if totalDatasetTokens >= maxDatasetTokens && !abQuiet {
					fmt.Printf("⚠ WARNING: Project has %d tokens of dataset summaries (recommended max: %d)\n", totalDatasetTokens, maxDatasetTokens)
					fmt.Printf("   Continuing will likely exceed local LLM context windows. Consider using --retrieval mode.\n\n")
				}

				outDir := filepath.Join(p.RootDir(), "dataset_summaries")
				if err := os.MkdirAll(outDir, 0o755); err != nil {
					return err
				}
				base := filepath.Base(path)
				safe := strings.TrimSuffix(base, filepath.Ext(base))
				sheetBase := safe
				if abSheetName != "" {
					s := strings.ToLower(strings.TrimSpace(abSheetName))
					var b strings.Builder
					for _, r := range s {
						if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
							b.WriteRune(r)
						} else if r == ' ' || r == '-' || r == '_' {
							b.WriteRune('-')
						}
					}
					ss := strings.Trim(b.String(), "-")
					if ss == "" {
						ss = "sheet"
					}
					sheetBase = safe + "__sheet-" + ss
				}
				outFile := filepath.Join(outDir, sheetBase+".summary.md")
				if _, statErr := os.Stat(outFile); statErr == nil {
					idx := 2
					for {
						cand := filepath.Join(outDir, fmt.Sprintf("%s__%d.summary.md", sheetBase, idx))
						if _, err := os.Stat(cand); os.IsNotExist(err) {
							if !abQuiet {
								fmt.Printf("⚠ Detected existing summary, writing to %s to avoid overwrite.\n", filepath.Base(cand))
							}
							outFile = cand
							break
						}
						idx++
					}
				}
				if err := os.WriteFile(outFile, []byte(md), 0o644); err != nil {
					return fmt.Errorf("write project summary: %w", err)
				}
				desc := abDescription
				if desc == "" {
					desc = "Auto-generated dataset summary"
				}
				if err := p.AddDocument(outFile, desc); err != nil {
					return err
				}
				if err := p.Save(); err != nil {
					return err
				}
				if !abQuiet {
					fmt.Printf("✓ Added analysis to project '%s' as %s\n", p.Name, filepath.Base(outFile))
				}
				written = true
			}
			if !written {
				if !abQuiet {
					fmt.Println(md)
				}
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(analyzeBatchCmd)
	analyzeBatchCmd.Flags().StringVarP(&abProject, "project", "p", "", "project name to attach summaries")
	analyzeBatchCmd.Flags().StringVar(&abDescription, "desc", "", "description when attaching to project")
	analyzeBatchCmd.Flags().StringVar(&abDelimiter, "delimiter", "", "CSV delimiter: ',' | ';' | 'tab'")
	analyzeBatchCmd.Flags().StringVar(&abDecimal, "decimal", "", "decimal separator for numbers: '.'|'comma' (auto-detect if omitted)")
	analyzeBatchCmd.Flags().StringVar(&abThousands, "thousands", "", "thousands separator for numbers: ','|'.'|'space' (auto-detect if omitted)")
	analyzeBatchCmd.Flags().IntVar(&abSampleRows, "sample-rows", 5, "number of sample rows to include")
	analyzeBatchCmd.Flags().IntVar(&abMaxRows, "max-rows", 100000, "maximum rows to process (0 = unlimited)")
	analyzeBatchCmd.Flags().StringSliceVar(&abGroupBy, "group-by", nil, "comma-separated column names to group by (repeatable)")
	analyzeBatchCmd.Flags().BoolVar(&abCorr, "correlations", false, "compute Pearson correlations among numeric columns")
	analyzeBatchCmd.Flags().BoolVar(&abCorrGroups, "corr-per-group", false, "compute correlation pairs within each group (may be slower)")
	analyzeBatchCmd.Flags().BoolVar(&abOutliers, "outliers", true, "compute robust outlier counts (MAD)")
	analyzeBatchCmd.Flags().Float64Var(&abOutlierThr, "outlier-threshold", 3.5, "robust |z| threshold for outliers (MAD-based)")
	analyzeBatchCmd.Flags().StringVar(&abSheetName, "sheet-name", "", "XLSX: sheet name to analyze")
	analyzeBatchCmd.Flags().IntVar(&abSheetIndex, "sheet-index", 1, "XLSX: 1-based sheet index (used if --sheet-name not provided)")
	analyzeBatchCmd.Flags().IntVar(&abSampleRowsProject, "sample-rows-project", -1, "when attaching (-p), override sample rows for dataset summaries (0 disables samples)")
	analyzeBatchCmd.Flags().BoolVar(&abQuiet, "quiet", false, "suppress progress and non-essential output")
}
