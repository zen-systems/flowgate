# flowgate

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

## Pipelines

Run the example pipeline locally (uses the mock adapter and command gates):

```bash
flowgate run -f pipelines/examples/feature.yaml -i "add a simple config loader"
```

Evidence bundles are written to:

```
.flowgate/runs/<run-id>/
```

By default, stages with `apply: true` run in a temp workspace; use `--apply` to persist changes:

```bash
flowgate run -f pipelines/examples/feature.yaml -i "add a simple config loader" --apply
```

You can override the base directory with `--out`:

```bash
flowgate run -f pipelines/examples/refactor.yaml -i "refactor router" --out /tmp/flowgate-runs
```

## Attestations

Generate a v0 attestation JSON from an existing run:

```bash
flowgate attest --run .flowgate/runs/<run-id> --stage implement --out /tmp/attestation.json
```

## Pipeline Example

```yaml
name: feature-implementation

workspace:
  path: .

gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]
    workdir: .
    allowed_commands: ["go test"]
    deny_shell: true

stages:
  - name: research
    adapter: mock
    model: mock-1
    prompt: "Research best practices for {{ .Input }}"

  - name: outline
    adapter: mock
    model: mock-1
    prompt: "Create an outline based on: {{ .Artifacts.research.Text }}"

  - name: implement
    adapter: mock
    model: mock-1
    prompt: "Implement this outline: {{ .Artifacts.outline.Text }}"
    gates:
      - go_test
    max_retries: 1
```

## Gates and Repair Loops

- Gates run in order for each stage.
- If a gate fails or errors, the stage fails closed by default.
  - If `max_retries` is set, a repair prompt is generated using gate feedback and the stage is retried.
  - Command gates capture stdout, stderr, exit code, and duration in the evidence bundle.
  - Command gates can restrict execution with `allowed_commands` and `deny_shell` (default true).

## Evidence Bundle

Each run records:

- run metadata (timestamp, pipeline path, input hash)
- per-stage prompt and output (or hashes if large)
- gate results and command outputs
- repair attempts and outcomes

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

See `configs/routing.yaml` for an example routing configuration.
