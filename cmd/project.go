package cmd

import (
	"fmt"

	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/spf13/cobra"
)

var (
	pmProject string
	pmClear   bool
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage per-project settings",
}

var projectSetModelCmd = &cobra.Command{
	Use:   "set-model <model>",
	Short: "Set or clear a project's default model",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if pmProject == "" {
			return fmt.Errorf("--project is required")
		}
		dir, err := resolveProjectDirByName(pmProject)
		if err != nil {
			return err
		}
		p, err := project.LoadProject(dir)
		if err != nil {
			return err
		}
		if p.Config == nil {
			p.Config = &project.ProjectConfig{}
		}
		if pmClear {
			p.Config.Model = ""
		} else {
			if len(args) == 0 || args[0] == "" {
				return fmt.Errorf("model is required unless --clear is set")
			}
			p.Config.Model = args[0]
		}
		if err := p.Save(); err != nil {
			return err
		}
		if pmClear {
			fmt.Printf("✓ Cleared project model for %s\n", pmProject)
		} else {
			fmt.Printf("✓ Set project model for %s: %s\n", pmProject, p.Config.Model)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectSetModelCmd)

	projectSetModelCmd.Flags().StringVarP(&pmProject, "project", "p", "", "project name")
	projectSetModelCmd.Flags().BoolVar(&pmClear, "clear", false, "clear the project's model override")
}
