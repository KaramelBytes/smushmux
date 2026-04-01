package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/KaramelBytes/smushmux/internal/parser"
	"github.com/KaramelBytes/smushmux/internal/utils"
	"github.com/google/uuid"
)

const (
	projectFileName = "project.json"
)

// Project represents a SmushMux project persisted on disk.
type Project struct {
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Instructions string               `json:"instructions"`
	Documents    map[string]*Document `json:"documents"`
	Config       *ProjectConfig       `json:"config"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`

	// Not serialized: on-disk location of the project.json
	rootDir string `json:"-"`
}

type ProjectConfig struct {
	Model       string  `json:"model"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// NewProject constructs an in-memory project. Call Save() to persist.
func NewProject(name, description, rootDir string) *Project {
	return &Project{
		Name:        name,
		Description: description,
		Documents:   make(map[string]*Document),
		// Leave Config fields empty to inherit from global defaults unless explicitly set per project.
		Config:    &ProjectConfig{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		rootDir:   rootDir,
	}
}

// LoadProject loads a project.json from the provided directory.
func LoadProject(dir string) (*Project, error) {
	path := filepath.Join(dir, projectFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("project not found at %s: %w", path, err)
		}
		return nil, fmt.Errorf("read project: %w", err)
	}
	var p Project
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}
	p.rootDir = dir
	return &p, nil
}

// RootDir returns the on-disk project directory path.
func (p *Project) RootDir() string { return p.rootDir }

// Save writes project.json using atomic write.
func (p *Project) Save() error {
	if p.rootDir == "" {
		return errors.New("project root directory not set")
	}
	if err := utils.EnsureProjectDir(p.rootDir); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}
	p.UpdatedAt = time.Now()
	data, err := utils.PrettyJSON(p)
	if err != nil {
		return err
	}
	return utils.SafeWriteFile(filepath.Join(p.rootDir, projectFileName), data)
}

// AddDocument reads a file and adds it to the project metadata and cache.
func (p *Project) AddDocument(path, description string) error {
	// Normalize path for comparison
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Check for duplicate paths
	for id, existing := range p.Documents {
		existingAbs, _ := filepath.Abs(existing.Path)
		if existingAbs == absPath {
			return fmt.Errorf("document already exists in project: %s\n  ID: %s\n  Description: %s\n  Use 'smushmux list --docs -p <project>' to view all documents",
									existing.Name, id, existing.Description)
						}
					}
				
					// Calculate current total tokens
					totalTokens := 0
					for _, doc := range p.Documents {
						totalTokens += doc.Tokens
					}
				
					// Parse new document
					parsed, err := parser.ParseFile(path)
					if err != nil {
						return fmt.Errorf("parse document: %w", err)
					}
				
					newTokens := parser.EstimateTokens(parsed)
					projectedTotal := totalTokens + newTokens
				
					// Enforce hard limit for projects targeting local LLMs
					const maxRecommendedTokens = 100000
					const maxCriticalTokens = 200000
				
					if projectedTotal > maxCriticalTokens {
						return fmt.Errorf("cannot add document: would exceed maximum project size (%d tokens). Current: %d, New: %d. Consider using --retrieval mode or creating separate projects",
							maxCriticalTokens, totalTokens, newTokens)
					}
				
					if projectedTotal > maxRecommendedTokens {
						fmt.Printf("⚠ WARNING: Total document content will be ~%d tokens (exceeds recommended %d).\n",
							projectedTotal, maxRecommendedTokens)
						fmt.Printf("   Consider: (1) Using --retrieval mode, (2) Reducing --max-rows for tabular files, or (3) Removing documents\n")
					}
				
					info, err := os.Stat(path)
					if err != nil {
						return fmt.Errorf("stat document: %w", err)
					}
					name := filepath.Base(path)
					id := uuid.NewString()
	d := &Document{
		ID:          id,
		Path:        path,
		Name:        name,
		Description: description,
		Content:     parsed,
		Tokens:      parser.EstimateTokens(parsed),
		AddedAt:     info.ModTime(),
	}
	if p.Documents == nil {
		p.Documents = make(map[string]*Document)
	}
	p.Documents[id] = d
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Project) SetInstructions(instructions string) {
	p.Instructions = strings.TrimSpace(instructions)
	p.UpdatedAt = time.Now()
}

// BuildPrompt assembles the final prompt text and returns the text with total token estimate.
func (p *Project) BuildPrompt() (string, int, error) {
	if p == nil {
		return "", 0, errors.New("project is nil")
	}
	if len(p.Documents) == 0 {
		return "", 0, errors.New("no documents added to project")
	}

	var sb strings.Builder
	// Header and instructions
	sb.WriteString("[INSTRUCTIONS]\n")
	sb.WriteString(p.Instructions)
	sb.WriteString("\n\n")
	// Relationships placeholder (future)
	sb.WriteString("[DOCUMENT RELATIONSHIPS]\n")
	sb.WriteString("(none)\n\n")
	// Reference documents
	sb.WriteString("[REFERENCE DOCUMENTS]\n")

	// deterministic order for stable prompts
	ids := make([]string, 0, len(p.Documents))
	for id := range p.Documents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		d := p.Documents[id]
		sb.WriteString("--- Document: ")
		sb.WriteString(d.Name)
		if d.Description != "" {
			sb.WriteString(" (")
			sb.WriteString(d.Description)
			sb.WriteString(")")
		}
		sb.WriteString(" ---\n")
		sb.WriteString(d.Content)
		sb.WriteString("\n\n")
	}

	// Task reiteration
	sb.WriteString("[TASK]\n")
	sb.WriteString("Follow the instructions above using the reference documents.\n")

	prompt := sb.String()
	tokens := utils.CountTokens(prompt)
	return prompt, tokens, nil
}
