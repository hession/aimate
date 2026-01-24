package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PromptConfig prompt configuration structure
type PromptConfig struct {
	Language string                     `yaml:"language"`
	Prompts  map[string]LanguagePrompts `yaml:"prompts"`
}

// LanguagePrompts prompts for a specific language
type LanguagePrompts struct {
	System        string `yaml:"system"`
	MemoryContext string `yaml:"memory_context"`
	ErrorPrefix   string `yaml:"error_prefix"`
}

// DefaultPromptConfig returns default prompt configuration
func DefaultPromptConfig() *PromptConfig {
	return &PromptConfig{
		Language: "zh",
		Prompts: map[string]LanguagePrompts{
			"zh": {
				System: `你是 AIMate，一个智能的 AI 工作伙伴。你可以帮助用户完成各种任务，包括：
- 回答问题和提供建议
- 读取和写入文件
- 执行系统命令
- 搜索文件内容

当用户让你"记住"某些信息时，请明确告诉用户你已经记住了，并在回复中确认这一点。

请使用友好、专业的语气与用户交流。在执行可能有风险的操作前，请先向用户确认。`,
				MemoryContext: "以下是你之前记住的相关信息：",
				ErrorPrefix:   "错误",
			},
			"en": {
				System: `You are AIMate, an intelligent AI work companion. You can help users complete various tasks, including:
- Answering questions and providing suggestions
- Reading and writing files
- Executing system commands
- Searching file contents

When users ask you to "remember" something, please explicitly confirm that you have remembered it in your response.

Please communicate with users in a friendly and professional manner. Before performing potentially risky operations, please confirm with the user first.`,
				MemoryContext: "Here is the relevant information you remembered earlier:",
				ErrorPrefix:   "Error",
			},
		},
	}
}

// PromptConfigPath returns the prompt config file path
func PromptConfigPath() (string, error) {
	// First check if there's a config/prompt.yaml in current working directory
	cwd, err := os.Getwd()
	if err == nil {
		localPath := filepath.Join(cwd, "config", "prompt.yaml")
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}

	// Fall back to user config directory
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "prompt.yaml"), nil
}

// LoadPromptConfig loads prompt configuration from file
func LoadPromptConfig() (*PromptConfig, error) {
	configPath, err := PromptConfigPath()
	if err != nil {
		return DefaultPromptConfig(), nil
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultPromptConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt config: %w", err)
	}

	// Parse config
	cfg := DefaultPromptConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse prompt config: %w", err)
	}

	return cfg, nil
}

// GetPrompts returns prompts for the configured language
func (p *PromptConfig) GetPrompts() LanguagePrompts {
	if prompts, ok := p.Prompts[p.Language]; ok {
		return prompts
	}
	// Fall back to Chinese if configured language not found
	if prompts, ok := p.Prompts["zh"]; ok {
		return prompts
	}
	return LanguagePrompts{}
}

// GetSystemPrompt returns the system prompt for the configured language
func (p *PromptConfig) GetSystemPrompt() string {
	return p.GetPrompts().System
}

// GetMemoryContext returns the memory context prefix for the configured language
func (p *PromptConfig) GetMemoryContext() string {
	return p.GetPrompts().MemoryContext
}

// GetErrorPrefix returns the error prefix for the configured language
func (p *PromptConfig) GetErrorPrefix() string {
	return p.GetPrompts().ErrorPrefix
}
