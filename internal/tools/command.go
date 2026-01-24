package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RunCommandTool run command tool
type RunCommandTool struct {
	confirmFunc func(command string) bool // Dangerous operation confirmation function
}

// NewRunCommandTool creates a new run command tool
func NewRunCommandTool(confirmFunc func(command string) bool) *RunCommandTool {
	return &RunCommandTool{
		confirmFunc: confirmFunc,
	}
}

func (t *RunCommandTool) Name() string {
	return "run_command"
}

func (t *RunCommandTool) Description() string {
	return "Execute a command in the shell. Can execute system commands, scripts, etc. Dangerous operations require user confirmation."
}

func (t *RunCommandTool) Parameters() []ParameterDef {
	return []ParameterDef{
		{
			Name:        "command",
			Type:        "string",
			Description: "The shell command to execute",
			Required:    true,
		},
		{
			Name:        "timeout",
			Type:        "number",
			Description: "Command timeout in seconds, default 30 seconds",
			Required:    false,
		},
	}
}

func (t *RunCommandTool) Execute(args map[string]any) (string, error) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("missing required parameter: command")
	}

	// Get timeout
	timeout := 30 * time.Second
	if to, ok := args["timeout"].(float64); ok && to > 0 {
		timeout = time.Duration(to) * time.Second
	}

	// Check if it's a dangerous operation
	if t.isDangerousCommand(command) {
		if t.confirmFunc != nil && !t.confirmFunc(command) {
			return "", fmt.Errorf("user cancelled dangerous operation")
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("$ %s\n\n", command))

	if stdout.Len() > 0 {
		result.WriteString(stdout.String())
	}

	if stderr.Len() > 0 {
		if stdout.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("STDERR:\n")
		result.WriteString(stderr.String())
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result.String(), fmt.Errorf("command execution timeout (%v)", timeout)
		}
		result.WriteString(fmt.Sprintf("\nExit status: %v", err))
	}

	return result.String(), nil
}

// isDangerousCommand checks if a command is dangerous
func (t *RunCommandTool) isDangerousCommand(command string) bool {
	dangerousPatterns := []string{
		"rm -rf",
		"rm -r",
		"rmdir",
		"dd if=",
		"> /dev/",
		"mkfs",
		"fdisk",
		"format",
		"shutdown",
		"reboot",
		"init 0",
		"init 6",
		":(){:|:&};:", // fork bomb
		"chmod -R 777",
		"chown -R",
		"wget", "curl", // Network downloads may be risky
	}

	lowerCmd := strings.ToLower(command)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerCmd, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
