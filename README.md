# piiPlug

PII (Personally Identifiable Information) plug-in system for Google ADK agents that automatically redact sensitive data from tool results and model responses, then unredacts it before passing arguments to tools.

## What it does

When an LLM agent interacts with your system — reading processes, running commands, querying APIs — the raw output often contains real usernames, email addresses, hostnames, or other PII. piiPlug intercepts that data at the ADK plugin hooks and replaces it with pronounceable fake names (driven by the APG algorithm), so the model sees safe synthetic data instead. When arguments flow back to a tool, they are unredacted so the system call uses the original values.


## Usage

### Programmatic (using the composite plugin)

```go
import "github.com/openSUSE/piiplug/piiplugin"

// All filters enabled
plug := piiplugin.NewPiiPlugin()

// Disable specific filters
plug = piiplugin.NewPiiPlugin(
    piiplugin.WithoutEmail(),
    piiplugin.WithoutHost(),
)

// Share a replacement table across multiple plugin instances
replacements := make(map[string]string)
plug = piiplugin.NewPiiPlugin(
    piiplugin.WithReplacement(&replacements),
)
```

### Using the agent demo

```bash
go run . --disable-username-plugin   # toggle individual filters at startup
```

Environment configuration:

| Variable | Default | Description |
|---|---|---|
| `OLLAMA_URL` | (set automatically) | Base URL for the Ollama API |
| `OLLAMA_MODEL` | `default:latest` | Model to use |


## Dependencies

- `google.golang.org/adk/v2` — Google Agent Development Kit
- `google.golang.org/genai` — Generative AI SDK
- `gorm.io/gorm` + `gorm.io/driver/sqlite` — Session storage
- `github.com/toon-format/toon-go` — TOON data format serialization

## License

MIT
