# Flowgate

AI orchestration system with intelligent routing, quality gates, and evidence bundles.

## The Problem

- Using AI for development means constantly switching between models
- Each model has different strengths, but remembering which to use is cognitive overhead
- AI outputs often look complete but contain stubs, mock data, and hollow implementations
- No systematic way to enforce quality before shipping

## The Solution

- **Automatic routing** based on task type detection
- **Quality gates** using hollowcheck and command-based tooling
- **Repair loops** that feed failures back to models
- **Multi-stage pipelines** with evidence bundles per run
- **Attestations** for supply chain verification

## Quick Start

```bash
go install github.com/zen-systems/flowgate@latest

# Set API keys (environment variables only for security)
export ANTHROPIC_API_KEY=sk-ant-...
export OPENAI_API_KEY=sk-...
export GOOGLE_API_KEY=AIza...
export DEEPSEEK_API_KEY=sk-...

# Ask a question (auto-routes to best model)
flowgate ask "Research tree-sitter Go bindings"
# → Routes to google/gemini-3-pro

flowgate ask "Implement a rate limiter in Go"
# → Routes to anthropic/claude-sonnet-4-20250514

# Override routing with model alias
flowgate ask --model quality "Review this architecture"

# See routing rules
flowgate routes

# See available models
flowgate models
```

## Pipelines

Run a multi-stage pipeline:

```bash
flowgate run --input "add user authentication" pipelines/feature.yaml
```

Evidence bundles are written to `~/.flowgate/runs/<run-id>/`.

Allow shell commands in gates:

```bash
flowgate run --yes --input "add unit tests" pipelines/feature.yaml
```

## Attestations

Generate and verify attestations for supply chain verification:

```bash
# Generate attestation from a run
flowgate attest <run-id> --output attestation.json

# Verify attestation integrity
flowgate verify attestation.json
```

## Pipeline Example

```yaml
name: feature-implementation

gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

stages:
  - name: research
    task_type: research
    prompt: "Research best practices for {{ .Input }}"

  - name: implement
    task_type: implement
    prompt: "Implement based on: {{ .Artifacts.research.Text }}"
    gates: [go_test]
    max_retries: 2
    apply: true
```

## Documentation

| Document | Description |
|----------|-------------|
| [CLI Reference](docs/CLI_REFERENCE.md) | Complete command and flag documentation |
| [Pipeline Manifests](docs/PIPELINE_MANIFESTS.md) | Pipeline YAML specification |
| [Routing](docs/ROUTING.md) | Intelligent routing configuration |
| [Quality Gates](docs/QUALITY_GATES.md) | Gate types, security, and repair loops |
| [Attestation](docs/ATTESTATION.md) | Evidence bundles and verification |
| [Workflows](docs/WORKFLOWS.md) | Common usage patterns and examples |
| [System Architecture](docs/SYSTEM.md) | Complete system documentation |

## Configuration

### Environment Variables (Required for API Keys)

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export GOOGLE_API_KEY="..."
export DEEPSEEK_API_KEY="..."
```

### Config Files

- `~/.flowgate/config.yaml` - General configuration
- `~/.flowgate/routing.yaml` - Custom routing rules
- `configs/models.yaml` - Model aliases

See [CLI Reference](docs/CLI_REFERENCE.md) for details.

## Security

- Shell execution is **denied by default** in command gates
- API keys must be set via **environment variables only**
- Workspace operations are **confined to the specified directory**
- All artifacts are **hashed with SHA-256** for attestation
