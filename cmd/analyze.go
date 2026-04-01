package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KaramelBytes/smushmux/internal/analysis"
	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/spf13/cobra"
)

var (
	anaProject     string
	anaOutputPath  string
	anaDescription string
	anaDelimiter   string
	anaSampleRows  int
	anaMaxRows     int
	anaGroupBy     []string
	anaCorr        bool
	anaSheetName   string
	anaSheetIndex  int
	anaCorrGroups  bool
	anaDecimal     string
	anaThousands   string
	anaOutliers    bool
	anaOutlierThr  float64
	anaSampleRowsProject int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze <file>",
	Short: "Analyze a CSV/TSV and produce a concise summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		opt := analysis.DefaultOptions()
		if anaSampleRows > 0 {
			opt.SampleRows = anaSampleRows
		}
		if anaMaxRows > 0 {
			opt.MaxRows = anaMaxRows
		}
		if anaDelimiter != "" {
			switch anaDelimiter {
			case ",":
				opt.Delimiter = ','
			case "\t", "tab":
				opt.Delimiter = '\t'
			case ";":
				opt.Delimiter = ';'
			default:
				return fmt.Errorf("unsupported --delimiter: %s", anaDelimiter)
			}
		}
		// Locale separators
		switch strings.ToLower(strings.TrimSpace(anaDecimal)) {
		case ",", "comma":
			opt.DecimalSeparator = ','
		case ".", "dot":
			opt.DecimalSeparator = '.'
		case "":
		default:
			return fmt.Errorf("unsupported --decimal: %s (use '.'|'comma')", anaDecimal)
		}
		switch strings.ToLower(strings.TrimSpace(anaThousands)) {
		case ",":
			opt.ThousandsSeparator = ','
		case ".":
			opt.ThousandsSeparator = '.'
		case "space", " ":
			opt.ThousandsSeparator = ' '
		case "":
		default:
			return fmt.Errorf("unsupported --thousands: %s (use ','|'.'|'space')", anaThousands)
		}
		// Analytics flags
		opt.GroupBy = anaGroupBy
		opt.Correlations = anaCorr
		opt.CorrPerGroup = anaCorrGroups
		if cmd.Flags().Changed("outliers") {
			opt.Outliers = anaOutliers
		} else {
			opt.Outliers = true
		}
		if anaOutlierThr > 0 {
			opt.OutlierThreshold = anaOutlierThr
		}
		if anaProject != "" && anaSampleRowsProject >= 0 {
			opt.SampleRows = anaSampleRowsProject
		}
		// choose analyzer by extension
		lower := strings.ToLower(path)
		var md string
		var err error
		if strings.HasSuffix(lower, ".xlsx") {
			rep, e := analysis.AnalyzeXLSX(path, opt, anaSheetName, anaSheetIndex)
			err = e
			if err == nil {
				md = rep.Markdown()
			}
		} else {
			rep, e := analysis.AnalyzeCSV(path, opt)
			err = e
			if err == nil {
				md = rep.Markdown()
			}
		}
		if err != nil {
			return err
		}

		// Decide where to write: --output path, or attach to project, or stdout
		written := false
		if anaOutputPath != "" {
			if err := os.WriteFile(anaOutputPath, []byte(md), 0o644); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			fmt.Printf("✓ Wrote analysis to %s\n", anaOutputPath)
			written = true
		}
		if anaProject != "" {
			projDir, err := resolveProjectDirByName(anaProject)
			if err != nil {
				return err
			}
			p, err := project.LoadProject(projDir)
			if err != nil {
				return err
			}

			// Count existing dataset summaries
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

			// Enforce limits
			const maxDatasetSummaries = 20
			const maxDatasetTokens = 150000

			if datasetCount >= maxDatasetSummaries {
				return fmt.Errorf("project already has %d dataset summaries (limit: %d).\n"+
					"  Consider: (1) Removing old summaries, (2) Using --retrieval mode, or (3) Creating a new project",
					datasetCount, maxDatasetSummaries)
			}

			if totalDatasetTokens >= maxDatasetTokens {
				fmt.Printf("⚠ WARNING: Project has %d tokens of dataset summaries (recommended max: %d)\n",
					totalDatasetTokens, maxDatasetTokens)
				fmt.Printf("   Continuing will likely exceed local LLM context windows. Consider using --retrieval mode.\n\n")
			}

			// Write summary as a doc file in project folder
			outDir := filepath.Join(p.RootDir(), "dataset_summaries")
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			base := filepath.Base(path)
			// ensure safe base for filename
			safe := strings.TrimSuffix(base, filepath.Ext(base))
			// disambiguate with sheet name if provided
			sheetBase := safe
			if anaSheetName != "" {
				s := strings.ToLower(strings.TrimSpace(anaSheetName))
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
						if !cmd.Flags().Changed("quiet") {
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
			desc := anaDescription
			if desc == "" {
				desc = "Auto-generated dataset summary"
			}
			if err := p.AddDocument(outFile, desc); err != nil {
				return err
			}
			if err := p.Save(); err != nil {
				return err
			}
			fmt.Printf("✓ Added analysis to project '%s' as %s\n", p.Name, filepath.Base(outFile))
			written = true
		}
		if !written {
			fmt.Println(md)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringVarP(&anaProject, "project", "p", "", "project name to attach summary")
	analyzeCmd.Flags().StringVarP(&anaOutputPath, "output", "o", "", "optional path to write analysis (Markdown)")
	analyzeCmd.Flags().StringVar(&anaDescription, "desc", "", "description when attaching to project")
	analyzeCmd.Flags().StringVar(&anaDelimiter, "delimiter", "", "CSV delimiter: ',' | ';' | 'tab'")
	analyzeCmd.Flags().StringVar(&anaDecimal, "decimal", "", "decimal separator for numbers: '.'|'comma' (auto-detect if omitted)")
	analyzeCmd.Flags().StringVar(&anaThousands, "thousands", "", "thousands separator for numbers: ','|'.'|'space' (auto-detect if omitted)")
	analyzeCmd.Flags().IntVar(&anaSampleRows, "sample-rows", 5, "number of sample rows to include")
	analyzeCmd.Flags().IntVar(&anaMaxRows, "max-rows", 100000, "maximum rows to process (0 = unlimited)")
	analyzeCmd.Flags().StringSliceVar(&anaGroupBy, "group-by", nil, "comma-separated column names to group by (repeatable)")
	analyzeCmd.Flags().BoolVar(&anaCorr, "correlations", false, "compute Pearson correlations among numeric columns")
	analyzeCmd.Flags().BoolVar(&anaCorrGroups, "corr-per-group", false, "compute correlation pairs within each group (may be slower)")
	analyzeCmd.Flags().BoolVar(&anaOutliers, "outliers", true, "compute robust outlier counts (MAD)")
	analyzeCmd.Flags().Float64Var(&anaOutlierThr, "outlier-threshold", 3.5, "robust |z| threshold for outliers (MAD-based)")
	analyzeCmd.Flags().StringVar(&anaSheetName, "sheet-name", "", "XLSX: sheet name to analyze")
	analyzeCmd.Flags().IntVar(&anaSheetIndex, "sheet-index", 1, "XLSX: 1-based sheet index (used if --sheet-name not provided)")
	analyzeCmd.Flags().IntVar(&anaSampleRowsProject, "sample-rows-project", -1, "when attaching (-p), override sample rows for dataset summaries (0 disables samples)")
}
