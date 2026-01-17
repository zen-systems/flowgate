# flowgate

AI orchestration system with intelligent routing and quality gates.

## The Problem

- Using AI for development means constantly switching between models
- Each model has different strengths, but remembering which to use is cognitive overhead
- AI outputs often look complete but contain stubs, mock data, and hollow implementations
- No systematic way to enforce quality before shipping

## The Solution

- **Automatic routing** based on task type detection
- **Quality gates** using hollowcheck integration
- **Repair loops** that feed failures back to models
- **Multi-model pipelines** for complex workflows

## Quick Start

```bash
go install github.com/zen-systems/flowgate@latest

# Set API keys
export ANTHROPIC_API_KEY=sk-ant-...
export OPENAI_API_KEY=sk-...
export GOOGLE_API_KEY=AIza...
export DEEPSEEK_API_KEY=sk-...

# Or use config file
mkdir -p ~/.flowgate
cat > ~/.flowgate/config.yaml << 'EOF'
api_keys:
  anthropic: "sk-ant-..."
  openai: "sk-..."
  google: "AIza..."
  deepseek: "sk-..."
EOF

# Ask a question (auto-routes to best model)
flowgate ask "Research tree-sitter Go bindings"
# → Routes to google/gemini-2.0-pro

flowgate ask "Implement a rate limiter in Go"
# → Routes to anthropic/claude-sonnet-4-20250514

# Override routing
flowgate ask --adapter openai --model gpt-5.2-codex "Scaffold a REST API"

# See routing rules
flowgate routes
```

## Model Routing Philosophy

Use fast/cheap models for volume work, quality models for integration and review, premium models for high-stakes decisions. Gate everything.

| Task Type | Model | Why |
|-----------|-------|-----|
| Research, long documents | Gemini 2.0 Pro | Best-in-class long context (1M+), strong at synthesis |
| Outlining, schema design | GPT-5.2 Instant | Fast, reliable JSON/schema compliance |
| Scaffolding, boilerplate | GPT-5.2 Codex | Purpose-built for code generation, 400K context |
| Large refactors, migrations | GPT-5.2 Codex | 400K context holds entire codebases |
| Implementation, integration | Claude Sonnet 4 | Refuses to cut corners, nuanced integration |
| Debugging | Claude Sonnet 4 | Understands intent, not just pattern matching |
| Code review | Claude Sonnet 4 | Catches subtle issues, maintains consistency |
| Security review | Claude Opus 4 | High-stakes decisions need best reasoning |
| Architecture, system design | Claude Opus 4 | Complex tradeoffs require deep reasoning |
| Legal, compliance | Claude Opus 4 | Can't afford mistakes, nuance matters |
| Math, proofs | GPT-5.2 Pro | 100% on AIME 2025, best benchmark performance |
| Bulk code generation | DeepSeek Coder | Aggressive pricing, gates catch issues anyway |
| Complex reasoning | DeepSeek Reasoner | R1 competitive with o1 at fraction of cost |

## Quality Gates

Flowgate integrates with [hollowcheck](https://github.com/zen-systems/hollowcheck) to enforce quality:

```yaml
# In pipeline definition
stages:
  - name: implement
    adapter: openai
    model: gpt-5.2-codex
    gate:
      type: hollowcheck
      contract: contracts/implementation.yaml
      threshold: 25
    on_fail:
      strategy: repair
      max_attempts: 2
```

When a gate fails, flowgate:

1. Extracts violations (TODOs, stubs, mock data)
2. Generates a repair prompt with specific fixes needed
3. Sends back to the model (or escalates to a different model)
4. Re-evaluates until passing or max attempts reached

Example repair hints generated from violations:

```
- Remove TODO comment at service.go:47
- Replace panic("not implemented") with real implementation at handler.go:23
- Replace placeholder/mock data at config.go:15
```

## Pipeline Example

```yaml
name: feature-implementation

stages:
  - name: research
    prompt: "Research best practices for {{ .input }}"
    # Auto-routes to Gemini

  - name: outline
    prompt: "Create implementation outline based on: {{ .artifacts.research }}"
    # Auto-routes to GPT-5.2

  - name: implement
    adapter: anthropic
    model: claude-sonnet-4-20250514
    prompt: "Implement this outline: {{ .artifacts.outline }}"
    gate:
      type: hollowcheck
      contract: contracts/implementation.yaml
    on_fail:
      strategy: escalate
      escalate_to:
        adapter: anthropic
        model: claude-opus-4-20250514

  - name: review
    adapter: anthropic
    model: claude-sonnet-4-20250514
    prompt: |
      Review this implementation:
      {{ .artifacts.implement }}

      Check for correctness, performance, and security issues.
```

Run pipelines:

```bash
flowgate run pipelines/feature.yaml --input "rate limiter with token bucket"
flowgate validate pipelines/feature.yaml  # Check without executing
```

## Configuration

### Config File (`~/.flowgate/config.yaml`)

```yaml
api_keys:
  anthropic: "sk-ant-..."
  openai: "sk-..."
  google: "AIza..."
  deepseek: "sk-..."
```

### Custom Routing (`~/.flowgate/routing.yaml`)

```yaml
task_types:
  research:
    triggers: ["research", "find", "look up", "what is", "compare"]
    adapter: google
    model: gemini-2.0-pro

  implement:
    triggers: ["implement", "code", "write a function", "build", "create"]
    adapter: anthropic
    model: claude-sonnet-4-20250514

  security_review:
    triggers: ["security", "vulnerability", "audit security", "penetration"]
    adapter: anthropic
    model: claude-opus-4-20250514

  bulk_code:
    triggers: ["bulk code", "generate multiple", "batch generate"]
    adapter: deepseek
    model: deepseek-coder

  reasoning:
    triggers: ["reason", "think through", "step by step", "logical"]
    adapter: deepseek
    model: deepseek-reasoner

default:
  adapter: anthropic
  model: claude-sonnet-4-20250514
```

## CLI Reference

```bash
# Route and execute
flowgate ask "Research tree-sitter Go bindings"

# Override routing
flowgate ask --adapter openai --model gpt-5.2-codex "Scaffold a REST API"

# Show routing rules
flowgate routes

# List available models (based on configured API keys)
flowgate models

# Execute pipeline
flowgate run pipelines/feature.yaml --input requirements.md

# Validate pipeline without executing
flowgate validate pipelines/feature.yaml

# Use custom routing config
flowgate --config ./custom-routing.yaml ask "Research something"
```

## Why This Exists

AI-generated code often satisfies the surface of a prompt while deferring actual implementation. Models produce "technically complete" outputs with TODOs, stub functions, and mock data.

The labs will fix model collapse. Nobody's fixing the quality problem except you.

Flowgate + hollowcheck creates an assembly line: fast models for volume, quality models for integration, gates to keep everyone honest. The models are interchangeable. The quality standards aren't.

## Project Structure

```
flowgate/
├── cmd/flowgate/main.go      # CLI entry point
├── pkg/
│   ├── adapter/              # LLM provider adapters
│   │   ├── anthropic.go      # Claude API
│   │   ├── openai.go         # OpenAI API
│   │   ├── google.go         # Gemini API
│   │   └── deepseek.go       # DeepSeek API
│   ├── router/               # Task routing
│   │   ├── router.go         # Router interface
│   │   └── rules.go          # Pattern matching
│   ├── gate/                 # Quality gates
│   │   └── hollowcheck.go    # Hollowcheck integration
│   ├── pipeline/             # Multi-stage pipelines
│   ├── repair/               # Repair prompt generation
│   └── config/               # Configuration loading
├── configs/
│   ├── routing.yaml          # Default routing rules
│   └── config.yaml.example   # Example config
└── pipelines/examples/       # Example pipelines
```

## Related Projects

- [hollowcheck](https://github.com/zen-systems/hollowcheck) - Quality gate for AI-generated code

## License

MIT
