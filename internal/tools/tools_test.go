package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test registration
	tool := NewReadFileTool()
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Test duplicate registration
	err = registry.Register(tool)
	if err == nil {
		t.Error("Duplicate registration should return error")
	}

	// Test get
	got, exists := registry.Get("read_file")
	if !exists {
		t.Error("Should be able to get registered tool")
	}
	if got.Name() != "read_file" {
		t.Errorf("Tool name mismatch: expected read_file, got %s", got.Name())
	}

	// Test get non-existent tool
	_, exists = registry.Get("not_exist")
	if exists {
		t.Error("Should not get unregistered tool")
	}
}

func TestReadFileTool(t *testing.T) {
	// Create temp file
	tmpDir, err := os.MkdirTemp("", "aimate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, AIMate!"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tool := NewReadFileTool()

	// Test normal read
	result, err := tool.Execute(map[string]any{"path": testFile})
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if result != testContent {
		t.Errorf("File content mismatch: expected %s, got %s", testContent, result)
	}

	// Test read non-existent file
	_, err = tool.Execute(map[string]any{"path": "/not/exist/file.txt"})
	if err == nil {
		t.Error("Reading non-existent file should return error")
	}

	// Test missing parameter
	_, err = tool.Execute(map[string]any{})
	if err == nil {
		t.Error("Missing parameter should return error")
	}
}

func TestWriteFileTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tool := NewWriteFileTool()
	testFile := filepath.Join(tmpDir, "output.txt")
	testContent := "Test content"

	// Test write
	result, err := tool.Execute(map[string]any{
		"path":    testFile,
		"content": testContent,
	})
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if !strings.Contains(result, "Successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify file content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != testContent {
		t.Errorf("File content mismatch: expected %s, got %s", testContent, string(content))
	}

	// Test write to subdirectory (auto-create)
	subDirFile := filepath.Join(tmpDir, "subdir", "file.txt")
	_, err = tool.Execute(map[string]any{
		"path":    subDirFile,
		"content": "test",
	})
	if err != nil {
		t.Fatalf("Failed to write file in subdirectory: %v", err)
	}
}

func TestListDirTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("test"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := NewListDirTool()

	result, err := tool.Execute(map[string]any{"path": tmpDir})
	if err != nil {
		t.Fatalf("Failed to list directory: %v", err)
	}

	if !strings.Contains(result, "file1.txt") {
		t.Error("Result should contain file1.txt")
	}
	if !strings.Contains(result, "file2.txt") {
		t.Error("Result should contain file2.txt")
	}
	if !strings.Contains(result, "subdir") {
		t.Error("Result should contain subdir")
	}
}

func TestSearchFilesTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aimate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("Hello World\nThis is a test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\nfunc main() {}"), 0644)

	tool := NewSearchFilesTool()

	// Search for existing content
	result, err := tool.Execute(map[string]any{
		"pattern": "Hello",
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if !strings.Contains(result, "hello.txt") {
		t.Error("Result should contain hello.txt")
	}

	// Search for non-existing content
	result, err = tool.Execute(map[string]any{
		"pattern": "NotFound12345",
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No content") {
		t.Error("Should return not found message")
	}
}

func TestRunCommandTool(t *testing.T) {
	tool := NewRunCommandTool(nil)

	// Test simple command
	result, err := tool.Execute(map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("Result should contain 'hello': %s", result)
	}

	// Test failed command
	_, err = tool.Execute(map[string]any{"command": "false"})
	// false command returns non-zero exit code, but shouldn't return error
	// just shows exit status in result

	// Test missing parameter
	_, err = tool.Execute(map[string]any{})
	if err == nil {
		t.Error("Missing parameter should return error")
	}
}

func TestGetSchemas(t *testing.T) {
	registry := NewDefaultRegistry(nil, nil)
	schemas := registry.GetSchemas()

	if len(schemas) != 7 {
		t.Errorf("Expected 7 tool schemas, got %d", len(schemas))
	}

	// Verify schema format
	for _, schema := range schemas {
		if schema.Type != "function" {
			t.Errorf("Schema type should be function, got %s", schema.Type)
		}
		if schema.Function.Name == "" {
			t.Error("Schema function name should not be empty")
		}
		if schema.Function.Description == "" {
			t.Error("Schema function description should not be empty")
		}
	}
}
