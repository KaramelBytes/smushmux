package cmd

import (
	"fmt"

	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/spf13/cobra"
)

var (
	instrProjectName string
)

var instructCmd = &cobra.Command{
	Use:   "instruct <instructions>",
	Short: "Set or update project instructions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		if instrProjectName == "" {
			return fmt.Errorf("--project is required")
		}
		projDir, err := resolveProjectDirByName(instrProjectName)
		if err != nil {
			return err
		}
		p, err := project.LoadProject(projDir)
		if err != nil {
			return err
		}
		p.SetInstructions(text)
		if err := p.Save(); err != nil {
			return err
		}
		fmt.Println("✓ Instructions updated")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(instructCmd)
	instructCmd.Flags().StringVarP(&instrProjectName, "project", "p", "", "project name")
}
