package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool read file tool
type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the content of a file at the specified path. Used to view the complete content of a file."
}

func (t *ReadFileTool) Parameters() []ParameterDef {
	return []ParameterDef{
		{
			Name:        "path",
			Type:        "string",
			Description: "The file path to read (supports absolute and relative paths)",
			Required:    true,
		},
	}
}

func (t *ReadFileTool) Execute(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing required parameter: path")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// WriteFileTool write file tool
type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file at the specified path. Creates the file if it doesn't exist, overwrites if it does."
}

func (t *WriteFileTool) Parameters() []ParameterDef {
	return []ParameterDef{
		{
			Name:        "path",
			Type:        "string",
			Description: "The file path to write to",
			Required:    true,
		},
		{
			Name:        "content",
			Type:        "string",
			Description: "The content to write to the file",
			Required:    true,
		},
	}
}

func (t *WriteFileTool) Execute(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing required parameter: path")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing required parameter: content")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote file: %s (%d bytes)", absPath, len(content)), nil
}

// ListDirTool list directory tool
type ListDirTool struct{}

func NewListDirTool() *ListDirTool {
	return &ListDirTool{}
}

func (t *ListDirTool) Name() string {
	return "list_dir"
}

func (t *ListDirTool) Description() string {
	return "List all files and subdirectories in the specified directory."
}

func (t *ListDirTool) Parameters() []ParameterDef {
	return []ParameterDef{
		{
			Name:        "path",
			Type:        "string",
			Description: "The directory path to list, defaults to current directory",
			Required:    false,
		},
	}
}

func (t *ListDirTool) Execute(args map[string]any) (string, error) {
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Read directory
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory: %s\n\n", absPath))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		typeStr := "📄"
		sizeStr := fmt.Sprintf("%d B", info.Size())
		if entry.IsDir() {
			typeStr = "📁"
			sizeStr = "<DIR>"
		}

		result.WriteString(fmt.Sprintf("%s %s\t%s\n", typeStr, entry.Name(), sizeStr))
	}

	return result.String(), nil
}

// SearchFilesTool search files tool
type SearchFilesTool struct{}

func NewSearchFilesTool() *SearchFilesTool {
	return &SearchFilesTool{}
}

func (t *SearchFilesTool) Name() string {
	return "search_files"
}

func (t *SearchFilesTool) Description() string {
	return "Search for files containing specified text in the given directory. Supports recursive search."
}

func (t *SearchFilesTool) Parameters() []ParameterDef {
	return []ParameterDef{
		{
			Name:        "pattern",
			Type:        "string",
			Description: "The text pattern to search for",
			Required:    true,
		},
		{
			Name:        "path",
			Type:        "string",
			Description: "The starting directory for search, defaults to current directory",
			Required:    false,
		},
	}
}

func (t *SearchFilesTool) Execute(args map[string]any) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return "", fmt.Errorf("missing required parameter: pattern")
	}

	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	var results []string
	maxResults := 50 // Limit result count

	err = filepath.Walk(absPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore errors, continue traversal
		}

		if info.IsDir() {
			// Skip hidden directories and common ignore directories
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only search text files
		if !isTextFile(filePath) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		// Search pattern
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if strings.Contains(line, pattern) {
				if len(results) >= maxResults {
					return filepath.SkipAll
				}
				relPath, _ := filepath.Rel(absPath, filePath)
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, i+1, strings.TrimSpace(line)))
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No content containing '%s' found in %s", pattern, absPath), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Search results (searching '%s' in %s):\n\n", pattern, absPath))
	for _, r := range results {
		result.WriteString(r + "\n")
	}

	if len(results) >= maxResults {
		result.WriteString(fmt.Sprintf("\n... Results truncated, showing first %d matches", maxResults))
	}

	return result.String(), nil
}

// isTextFile checks if a file is a text file
func isTextFile(path string) bool {
	textExts := []string{
		".txt", ".md", ".go", ".py", ".js", ".ts", ".jsx", ".tsx",
		".html", ".css", ".json", ".yaml", ".yml", ".xml",
		".sh", ".bash", ".zsh", ".fish",
		".c", ".cpp", ".h", ".hpp", ".java", ".rs", ".rb",
		".php", ".sql", ".toml", ".ini", ".conf", ".cfg",
		".gitignore", ".dockerignore", ".env",
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}

	// Check common text files without extension
	name := filepath.Base(path)
	textNames := []string{"Makefile", "Dockerfile", "README", "LICENSE", "CHANGELOG"}
	for _, textName := range textNames {
		if name == textName {
			return true
		}
	}

	return false
}
