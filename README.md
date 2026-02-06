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

# Build using script
./build.sh

# Or build with specific options
./build.sh -d          # Build with debug symbols
./build.sh -r          # Build for release (optimized)
./build.sh -c          # Clean before build
./build.sh -a          # Build for all platforms (linux, darwin, windows)

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


# Web search configuration
web_search:
  provider: "duckduckgo"       # duckduckgo | searxng
  base_url: "https://api.duckduckgo.com"
  api_key: ""                  # optional, used by searxng instances
  timeout_seconds: 15
  default_limit: 5
  user_agent: "AIMate/0.1"
```

### 2. Start Chatting

```bash
# Run directly (development mode)
./cli.sh

# Or run with options
./cli.sh -d                 # Run with debug logging
./cli.sh -c ./myconfig      # Use custom config directory
./cli.sh -b -e              # Build first, then run binary
./cli.sh -- --help          # Pass args to aimate
```

Example session:
```
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

## 🛠️ Scripts

### `build.sh` - Build Script

```bash
./build.sh              # Build for current platform
./build.sh -d           # Build with debug symbols
./build.sh -r           # Build for release (optimized, smaller binary)
./build.sh -c           # Clean build directory before build
./build.sh -a           # Build for all platforms (darwin/linux/windows)
./build.sh -o myname    # Custom output binary name
./build.sh -h           # Show help
```

Features:
- Automatically runs tests before building
- Injects version info (git tag, commit, build time)
- Supports cross-compilation for multiple platforms
- Outputs to `bin/` directory

### `cli.sh` - Run Script

```bash
./cli.sh                # Run with go run (development)
./cli.sh -e             # Run the built binary from bin/
./cli.sh -b -e          # Build first, then run binary
./cli.sh -d             # Run with debug logging enabled
./cli.sh -c ./cfg       # Use custom config directory
./cli.sh -- --help      # Pass arguments to aimate
./cli.sh -h             # Show help
```

Features:
- Quick development mode with `go run`
- Option to run compiled binary
- Debug mode support
- Custom config directory support

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
| `search_web` | Search the web for fresh information |
| `fetch_url` | Fetch a URL for readable content |

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
│   ├── logger/          # Logging system
│   ├── memory/          # Memory storage system
│   └── tools/           # Tool system
├── logs/                # Log files (auto-created)
├── build.sh             # Build script
├── cli.sh               # Run script
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

## 📄 License

MIT License
