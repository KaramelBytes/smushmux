package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/spf13/cobra"
)

var (
	listProjects bool
	listDocs     bool
	listProjName string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects or documents",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listProjects == listDocs { // either both true or both false
			return fmt.Errorf("specify exactly one of --projects or --docs")
		}
		if listProjects {
			return listAllProjects()
		}
		// list docs
		if listProjName == "" {
			return fmt.Errorf("--project is required when using --docs")
		}
		projDir, err := resolveProjectDirByName(listProjName)
		if err != nil {
			return err
		}
		p, err := project.LoadProject(projDir)
		if err != nil {
			return err
		}
		if len(p.Documents) == 0 {
			fmt.Println("(no documents)")
			return nil
		}
		ids := make([]string, 0, len(p.Documents))
		for id := range p.Documents {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			d := p.Documents[id]
			fmt.Printf("- %s: %s (%s)\n", d.ID, d.Name, d.Description)
		}
		return nil
	},
}

func listAllProjects() error {
	root, err := defaultProjectsDir()
	if err != nil {
		return err
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	found := false
	for _, e := range dirs {
		if !e.IsDir() {
			continue
		}
		pj := filepath.Join(root, e.Name(), "project.json")
		if _, err := os.Stat(pj); err == nil {
			fmt.Printf("- %s\n", e.Name())
			found = true
		}
	}
	if !found {
		fmt.Println("(no projects)")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listProjects, "projects", false, "list projects")
	listCmd.Flags().BoolVar(&listDocs, "docs", false, "list documents in a project")
	listCmd.Flags().StringVarP(&listProjName, "project", "p", "", "project name for --docs")
}
