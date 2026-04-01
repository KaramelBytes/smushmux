package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KaramelBytes/smushmux/internal/project"
	"github.com/KaramelBytes/smushmux/internal/utils"
	"github.com/spf13/cobra"
)

var (
	initDescription string
)

var initCmd = &cobra.Command{
	Use:   "init <project-name>",
		Short: "Initialize a new SmushMux project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		root, err := defaultProjectsDir()
		if err != nil {
			return err
		}
		projDir := filepath.Join(root, name)
		// Refuse to overwrite an existing project.
		if info, err := os.Stat(projDir); err == nil && info.IsDir() {
			projectFile := filepath.Join(projDir, "project.json")
			if _, err := os.Stat(projectFile); err == nil {
				return fmt.Errorf("project already exists at %s", projDir)
			}
			entries, err := os.ReadDir(projDir)
			if err != nil {
				return fmt.Errorf("inspect project directory: %w", err)
			}
			if len(entries) > 0 {
				return fmt.Errorf("directory %s already exists and is not empty; refusing to initialize project", projDir)
			}
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stat project directory: %w", err)
		}
		if err := utils.EnsureProjectDir(projDir); err != nil {
			return err
		}
		p := project.NewProject(name, initDescription, projDir)
		if err := p.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Project initialized: %s\n", projDir)
		return nil
	},
}

func defaultProjectsDir() (string, error) {
	if cfg != nil && cfg.ProjectsDir != "" {
		dir := cfg.ProjectsDir
		if strings.HasPrefix(dir, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home dir: %w", err)
			}
			dir = strings.TrimPrefix(dir, "~")
			dir = strings.TrimPrefix(dir, string(os.PathSeparator))
			dir = strings.TrimPrefix(dir, "/")
			dir = filepath.Join(home, dir)
		}
		dir = filepath.Clean(dir)
		if err := utils.EnsureProjectDir(dir); err != nil {
			return "", err
		}
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, ".smushmux", "projects")
	if err := utils.EnsureProjectDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func resolveProjectDirByName(name string) (string, error) {
	if name == "" {
		return "", errors.New("project name is required")
	}
	root, err := defaultProjectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&initDescription, "desc", "d", "", "project description")
}
