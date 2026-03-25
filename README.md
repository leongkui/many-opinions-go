# Many Opinions MCP Server (Go)

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that provides diverse "second opinions" from multiple large language model providers (OpenAI, Anthropic, Gemini). It enables LLM clients to dynamically route questions, aggregate perspectives, and seek advice across different AI personalities and reasoning tiers.

## Features

- **Get a Second Opinion (`ask_opinion`)**: Ask a question or topic and get an opinion from a specific AI model. Configure the persona (e.g., `honest`, `friend`, `coach`, `wise`, `creative`).
- **Compare Opinions (`compare_opinions`)**: Broadcast a question to the top models from 3 distinct providers simultaneously and receive an aggregated comparison.
- **Collect Opinions (`collect_opinions`)**: Poll multiple AI models for opinions, picking from quality tiers.
- **Dynamic Model Catalog (`list_available_models`)**: Inspect configured models, their providers, and their corresponding capability quality tiers.
- **Stateless Operation**: Built via `mcp-go` with stateless HTTP support.

## Prerequisites

- Go 1.22+
- API keys for the providers you wish to utilize (OpenAI, Anthropic, Google Gemini)

## Quickstart

1. Configure Environment:
   ```bash
   cp .env.example .env
   # Edit .env to include OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY, etc.
   ```

2. Install dependencies:
   ```bash
   make install
   ```

## Usage

### Running Locally
You can run the server locally for testing or HTTP connections:
- `make run`: Run the server in the foreground.
- `make start`: Start the server in the background. (Use `make stop` to halt and `make logs` to tail).

### Configuring Models
The server dynamically loads available models from `models.json`. You can adjust the models, their display names, and their `quality` scores.

## MCP Client Configuration

To hook this up to an MCP client (such as Claude Desktop), use standard standard I/O (stdio) transport via `go`:

```json
{
  "mcpServers": {
    "many-opinions": {
      "command": "go",
      "args": [
        "run",
        "/absolute/path/to/many-opinions-go"
      ],
      "env": {
        "OPENAI_API_KEY": "your-key",
        "ANTHROPIC_API_KEY": "your-key",
        "GEMINI_API_KEY": "your-key"
      }
    }
  }
}
```

## License

MIT License. See `LICENSE` for details.
