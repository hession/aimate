package tools

// Tool tool interface
type Tool interface {
	Name() string                                // Tool name
	Description() string                         // Tool description (for LLM)
	Parameters() []ParameterDef                  // Parameter definitions
	Execute(args map[string]any) (string, error) // Execute
}

// ParameterDef parameter definition
type ParameterDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string" | "number" | "boolean"
	Description string `json:"description"`
	Required    bool   `json:"required"`
}
