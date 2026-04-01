package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KaramelBytes/smushmux/internal/project"
	cfgpkg "github.com/KaramelBytes/smushmux/internal/config"
)

func preserveLifecycleGlobals(t *testing.T) {
	t.Helper()
	origCfg := cfg
	origAddProjectName := addProjectName
	origListProjects := listProjects
	origListDocs := listDocs
	origListProjName := listProjName
	origPMProject := pmProject
	origPMClear := pmClear
	t.Cleanup(func() {
		cfg = origCfg
		addProjectName = origAddProjectName
		listProjects = origListProjects
		listDocs = origListDocs
		listProjName = origListProjName
		pmProject = origPMProject
		pmClear = origPMClear
	})
}

func TestDefaultProjectsDirHonorsConfigAndExpandsTilde(t *testing.T) {
	preserveLifecycleGlobals(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg = &cfgpkg.Global{ProjectsDir: "~/.custom-projects"}

	dir, err := defaultProjectsDir()
	if err != nil {
		t.Fatalf("defaultProjectsDir: %v", err)
	}
	want := filepath.Join(home, ".custom-projects")
	if dir != want {
		t.Fatalf("projects dir mismatch: got %q want %q", dir, want)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected projects dir to exist")
	}
}

func TestResolveProjectDirByNameRequiresName(t *testing.T) {
	preserveLifecycleGlobals(t)

	if _, err := resolveProjectDirByName(""); err == nil {
		t.Fatalf("expected error for empty project name")
	}
}

func TestAddCommandRequiresProjectFlag(t *testing.T) {
	preserveLifecycleGlobals(t)

	addProjectName = ""
	if err := addCmd.RunE(addCmd, []string{"/tmp/doc.md"}); err == nil {
		t.Fatalf("expected --project required error")
	}
}

func TestListCommandValidation(t *testing.T) {
	preserveLifecycleGlobals(t)

	listProjects = false
	listDocs = false
	if err := listCmd.RunE(listCmd, nil); err == nil {
		t.Fatalf("expected exactly one of --projects/--docs error")
	}

	listProjects = false
	listDocs = true
	listProjName = ""
	if err := listCmd.RunE(listCmd, nil); err == nil {
		t.Fatalf("expected --project required with --docs")
	}
}

func TestInitCommandRejectsExistingProject(t *testing.T) {
	preserveLifecycleGlobals(t)

	root := t.TempDir()
	cfg = &cfgpkg.Global{ProjectsDir: root}
	name := "demo"
	projDir := filepath.Join(root, name)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir projDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "project.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	if err := initCmd.RunE(initCmd, []string{name}); err == nil {
		t.Fatalf("expected existing project error")
	}
}

func TestProjectSetModelValidationAndClear(t *testing.T) {
	preserveLifecycleGlobals(t)

	root := t.TempDir()
	cfg = &cfgpkg.Global{ProjectsDir: root}
	p := project.NewProject("p1", "", filepath.Join(root, "p1"))
	if err := p.Save(); err != nil {
		t.Fatalf("save project: %v", err)
	}

	pmProject = ""
	pmClear = false
	if err := projectSetModelCmd.RunE(projectSetModelCmd, []string{"openai/gpt-4o-mini"}); err == nil {
		t.Fatalf("expected --project required error")
	}

	pmProject = "p1"
	pmClear = false
	if err := projectSetModelCmd.RunE(projectSetModelCmd, []string{}); err == nil {
		t.Fatalf("expected model required error when --clear is not set")
	}

	if err := projectSetModelCmd.RunE(projectSetModelCmd, []string{"openai/gpt-4o-mini"}); err != nil {
		t.Fatalf("set model: %v", err)
	}
	loaded, err := project.LoadProject(filepath.Join(root, "p1"))
	if err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if loaded.Config == nil || loaded.Config.Model != "openai/gpt-4o-mini" {
		t.Fatalf("expected model to be saved")
	}

	pmClear = true
	if err := projectSetModelCmd.RunE(projectSetModelCmd, []string{}); err != nil {
		t.Fatalf("clear model: %v", err)
	}
	loaded, err = project.LoadProject(filepath.Join(root, "p1"))
	if err != nil {
		t.Fatalf("reload project after clear: %v", err)
	}
	if loaded.Config == nil || loaded.Config.Model != "" {
		t.Fatalf("expected model to be cleared")
	}
}

func TestInitAddListAndProjectSetModelOutputs(t *testing.T) {
	preserveLifecycleGlobals(t)

	root := t.TempDir()
	cfg = &cfgpkg.Global{ProjectsDir: root}

	initOut, err := captureStdout(t, func() error {
		return initCmd.RunE(initCmd, []string{"demo"})
	})
	if err != nil {
		t.Fatalf("init command: %v", err)
	}
	if !strings.Contains(initOut, "Project initialized") {
		t.Fatalf("expected init success output, got %q", initOut)
	}

	doc := filepath.Join(root, "doc.md")
	if err := os.WriteFile(doc, []byte("# demo\n\ncontent"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	addProjectName = "demo"
	addOut, err := captureStdout(t, func() error {
		return addCmd.RunE(addCmd, []string{doc})
	})
	if err != nil {
		t.Fatalf("add command: %v", err)
	}
	if !strings.Contains(addOut, "Document added") {
		t.Fatalf("expected add success output, got %q", addOut)
	}

	listProjects = false
	listDocs = true
	listProjName = "demo"
	listOut, err := captureStdout(t, func() error {
		return listCmd.RunE(listCmd, nil)
	})
	if err != nil {
		t.Fatalf("list docs command: %v", err)
	}
	if !strings.Contains(listOut, "doc.md") {
		t.Fatalf("expected listed doc in output, got %q", listOut)
	}

	pmProject = "demo"
	pmClear = false
	setOut, err := captureStdout(t, func() error {
		return projectSetModelCmd.RunE(projectSetModelCmd, []string{"openai/gpt-4o-mini"})
	})
	if err != nil {
		t.Fatalf("project set-model command: %v", err)
	}
	if !strings.Contains(setOut, "Set project model") {
		t.Fatalf("expected set-model output, got %q", setOut)
	}

	pmClear = true
	clearOut, err := captureStdout(t, func() error {
		return projectSetModelCmd.RunE(projectSetModelCmd, []string{})
	})
	if err != nil {
		t.Fatalf("project set-model --clear command: %v", err)
	}
	if !strings.Contains(clearOut, "Cleared project model") {
		t.Fatalf("expected clear-model output, got %q", clearOut)
	}
}
