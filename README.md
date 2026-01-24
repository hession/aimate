# AIMate

🤖 **AIMate** - Your AI Work Companion

AIMate is an intelligent command-line AI assistant that understands your intent and helps complete various tasks. It's powered by DeepSeek LLM and supports natural language conversation, file operations, command execution, and more.

## ✨ Features

- 🗣️ **Natural Language Conversation** - Fluent dialogue with AI
- 📁 **File Operations** - Read, write, and search file contents
- 💻 **Command Execution** - Execute shell commands (dangerous operations require confirmation)
- 🧠 **Memory System** - Remember important information you tell it
- 🔧 **Tool Calling** - Automatically identify intent and call appropriate tools

## 📦 Installation

### Build from Source

Make sure you have Go 1.23+ installed:

```bash
# Clone the repository
git clone https://github.com/hession/aimate.git
cd aimate

# Build
go build -o bin/aimate ./cmd/aimate

# Install to system path (optional)
sudo mv bin/aimate /usr/local/bin/
```

## 🚀 Quick Start

### 1. Configure API Key

You can configure the API key in one of two ways:

**Option A: Using secrets file (recommended)**

Create `config/.secrets` file:
```
DEEPSEEK_API_KEY=your-api-key-here
```

**Option B: Using config file**

Edit `~/.aimate/config.yaml`:
```yaml
model:
  api_key: "your-api-key-here"
  base_url: "https://api.deepseek.com"
  model: "deepseek-chat"
  temperature: 0.7
  max_tokens: 4096
```

### 2. Start Chatting

```bash
$ aimate

🤖 AIMate v0.1.0 - Your AI Work Companion
Type /help for help, /exit to quit

You: Show me the files in current directory

AIMate: Let me check the current directory...

🔧 Calling tool: list_dir
   Args: {"path": "."}
   Status: ✅ Done

The current directory contains:
- main.go
- go.mod
- README.md
...
```

### 3. Configure Prompts

You can customize the system prompts by editing `config/prompt.yaml`:

```yaml
# Default language: zh (Chinese) or en (English)
language: zh

prompts:
  zh:
    system: |
      你是 AIMate，一个智能的 AI 工作伙伴...
  en:
    system: |
      You are AIMate, an intelligent AI work companion...
```

## 📚 Built-in Commands

| Command | Description |
|---------|-------------|
| `/help` | Show help information |
| `/clear` | Clear current session history |
| `/new` | Create new session |
| `/config` | Show current configuration |
| `/exit` | Exit program |

## 🔧 Available Tools

| Tool | Description |
|------|-------------|
| `read_file` | Read file content |
| `write_file` | Write file content |
| `list_dir` | List directory content |
| `run_command` | Execute shell command |
| `search_files` | Search file content |

## 💡 Usage Examples

### File Operations

```
You: Read the content of main.go
You: Create a file hello.txt with content "Hello World"
You: Search for files containing "TODO" in the project
```

### Command Execution

```
You: Run go test to see the test results
You: Show current git status
```

### Memory Feature

```
You: Remember that my project uses Go language
AIMate: Got it, I've remembered that your project uses Go language. ✅

You: What language did I tell you my project uses?
AIMate: You told me that your project uses Go language.
```

## 📁 Project Structure

```
aimate/
├── cmd/aimate/          # Program entry
├── config/              # Configuration files
│   ├── .secrets.example # Secrets template
│   └── prompt.yaml      # Prompt configuration
├── internal/
│   ├── agent/           # Agent core logic
│   ├── cli/             # CLI interface
│   ├── config/          # Configuration management
│   ├── llm/             # LLM client
│   ├── memory/          # Memory storage system
│   └── tools/           # Tool system
├── go.mod
├── go.sum
└── README.md
```

## ⚙️ Configuration

### Main Config (`~/.aimate/config.yaml`)

```yaml
# LLM model configuration
model:
  api_key: ""                          # DeepSeek API Key (can also use .secrets)
  base_url: "https://api.deepseek.com" # API endpoint
  model: "deepseek-chat"               # Model name
  temperature: 0.7                     # Temperature (0-2)
  max_tokens: 4096                     # Max tokens

# Memory configuration
memory:
  db_path: "~/.aimate/memory.db"       # Database path
  max_context_messages: 20             # Max context messages

# Safety configuration
safety:
  confirm_dangerous_ops: true          # Confirm dangerous operations
```

### Secrets (`config/.secrets`)

```
DEEPSEEK_API_KEY=your-api-key-here
```

### Prompts (`config/prompt.yaml`)

```yaml
language: zh  # or "en" for English

prompts:
  zh:
    system: "..."
    memory_context: "以下是你之前记住的相关信息："
    error_prefix: "错误"
  en:
    system: "..."
    memory_context: "Here is the relevant information you remembered earlier:"
    error_prefix: "Error"
```

## 🧪 Run Tests

```bash
go test ./...
```

## 📝 Version History

### v0.1.0 (MVP)

- ✅ Basic conversation (DeepSeek integration)
- ✅ Agent framework (custom, with tool calling)
- ✅ 5 core tools (read_file, write_file, list_dir, run_command, search_files)
- ✅ Local memory system (SQLite)
- ✅ CLI interface
- ✅ Configurable prompts (Chinese/English)
- ✅ Secrets management

## 🤝 Contributing

Issues and Pull Requests are welcome!

## 📄 License

MIT License
