package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/spf13/cobra"
)

var (
	addProjectName string
	addDocDesc     string
)

var addCmd = &cobra.Command{
	Use:   "add <file>",
	Short: "Add a document to a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file := args[0]
		if addProjectName == "" {
			return fmt.Errorf("--project is required")
		}
		projDir, err := resolveProjectDirByName(addProjectName)
		if err != nil {
			return err
		}
		p, err := project.LoadProject(projDir)
		if err != nil {
			return err
		}
		if err := p.AddDocument(file, addDocDesc); err != nil {
			return err
		}
		if err := p.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Document added: %s\n", filepath.Base(file))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addProjectName, "project", "p", "", "project name")
	addCmd.Flags().StringVar(&addDocDesc, "desc", "", "document description")
}
