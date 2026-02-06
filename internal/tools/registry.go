package tools

import (
	"fmt"
	"sync"

	"github.com/hession/aimate/internal/config"
)

// Registry tool registry
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register registers a tool
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already exists", name)
	}

	r.tools[name] = tool
	return nil
}

// Get gets a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// List lists all tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Execute executes a tool by name
func (r *Registry) Execute(name string, args map[string]any) (string, error) {
	tool, exists := r.Get(name)
	if !exists {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(args)
}

// ToolSchema tool schema (for Function Calling)
type ToolSchema struct {
	Type     string         `json:"type"`
	Function FunctionSchema `json:"function"`
}

// FunctionSchema function schema
type FunctionSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// GetSchemas gets all tool schemas for Function Calling
func (r *Registry) GetSchemas() []ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make([]ToolSchema, 0, len(r.tools))
	for _, tool := range r.tools {
		schema := ToolSchema{
			Type: "function",
			Function: FunctionSchema{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  buildParameterSchema(tool.Parameters()),
			},
		}
		schemas = append(schemas, schema)
	}
	return schemas
}

// buildParameterSchema builds parameter schema
func buildParameterSchema(params []ParameterDef) map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	for _, param := range params {
		properties[param.Name] = map[string]interface{}{
			"type":        param.Type,
			"description": param.Description,
		}
		if param.Required {
			required = append(required, param.Name)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// NewDefaultRegistry creates and registers all default tools
func NewDefaultRegistry(confirmFunc func(command string) bool, cfg *config.Config) *Registry {
	registry := NewRegistry()

	// Register all built-in tools
	tools := []Tool{
		NewReadFileTool(),
		NewWriteFileTool(),
		NewListDirTool(),
		NewRunCommandTool(confirmFunc),
		NewSearchFilesTool(),
		NewWebSearchTool(cfg),
		NewFetchURLTool(cfg),
	}

	for _, tool := range tools {
		_ = registry.Register(tool) // Ignore errors as we know these tool names won't conflict
	}

	return registry
}
