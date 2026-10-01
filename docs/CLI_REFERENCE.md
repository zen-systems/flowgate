# Flowgate CLI Reference

Flowgate is an intelligent AI workflow orchestrator that automatically routes prompts to the best-suited LLM, enforces quality gates, and generates auditable evidence bundles.

## Installation

```bash
go install github.com/yourusername/flowgate/cmd/flowgate@latest
```

## Environment Variables

API keys must be set via environment variables (not config files) for security:

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key for Claude models |
| `OPENAI_API_KEY` | OpenAI API key for GPT models |
| `GOOGLE_API_KEY` | Google API key for Gemini models |
| `DEEPSEEK_API_KEY` | DeepSeek API key for DeepSeek models |

## Commands

### `flowgate ask`

Execute a single-shot prompt with intelligent routing.

```bash
flowgate ask [flags] "your prompt here"
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--adapter` | string | (routed) | Force a specific adapter: `anthropic`, `openai`, `google`, `deepseek` |
| `--model` | string | (routed) | Force a specific model or alias |
| `--curator` | string | | Path to a curator prompt file for context injection |
| `--gates` | string | | Comma-separated list of gates to evaluate (e.g., `go_test,hollowcheck`) |
| `--config` | string | `~/.flowgate/config.yaml` | Path to config file |

**Examples:**

```bash
# Basic usage - routes automatically
flowgate ask "How do I implement a binary search in Go?"

# Force a specific model
flowgate ask --model claude-sonnet-4-20250514 "Review this code for security issues"

# Use model alias
flowgate ask --model fast "Quick question about syntax"

# With context curation
flowgate ask --curator ./context.md "Refactor the authentication module"

# With quality gates
flowgate ask --gates go_test "Write a function that validates email addresses"
```

---

### `flowgate run`

Execute a multi-stage pipeline defined in a YAML manifest.

```bash
flowgate run [flags] <manifest.yaml>
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--input` | string | | Input text to pass to the pipeline |
| `--input-file` | string | | Path to file containing input text |
| `--workspace` | string | `.` | Working directory for file operations |
| `--dry-run` | bool | `false` | Show what would be executed without running |
| `--yes` | bool | `false` | Allow shell command execution in gates |
| `--config` | string | `~/.flowgate/config.yaml` | Path to config file |

**Examples:**

```bash
# Run a feature implementation pipeline
flowgate run --input "Add user authentication" pipelines/feature.yaml

# Run with input from file
flowgate run --input-file requirements.txt pipelines/refactor.yaml

# Run in a specific workspace
flowgate run --workspace ./myproject --input "Optimize database queries" pipelines/feature.yaml

# Allow shell commands in gates
flowgate run --yes --input "Add unit tests" pipelines/feature.yaml

# Preview without execution
flowgate run --dry-run --input "Refactor logging" pipelines/refactor.yaml
```

---

### `flowgate validate`

Validate a pipeline manifest for correctness.

```bash
flowgate validate <manifest.yaml>
```

**Validation checks:**

- Required fields (`name`, `stages`)
- Unique stage names
- Valid gate references
- Template syntax correctness

**Example:**

```bash
flowgate validate pipelines/feature.yaml
```

---

### `flowgate attest`

Generate an attestation document from a completed pipeline run.

```bash
flowgate attest [flags] <run-id>
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | string | stdout | Output file path for attestation JSON |
| `--stage` | string | | Specific stage to attest (defaults to all) |
| `--config` | string | `~/.flowgate/config.yaml` | Path to config file |

**Examples:**

```bash
# Generate attestation for a run
flowgate attest abc123def

# Save to file
flowgate attest --output attestation.json abc123def

# Attest specific stage
flowgate attest --stage implement abc123def
```

---

### `flowgate verify`

Verify an attestation document's integrity.

```bash
flowgate verify <attestation.json>
```

**Verification checks:**

- All hash references match SHA-256 of actual content
- Claim gates match recorded stage gate results
- Evidence bundle completeness
- Schema version compatibility

**Example:**

```bash
flowgate verify attestation.json
```

---

### `flowgate routes`

Display configured routing rules and task types.

```bash
flowgate routes [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `~/.flowgate/config.yaml` | Path to config file |

**Output includes:**

- Task types and their trigger keywords
- Adapter/model mappings per task type
- Confidence thresholds
- Default fallback configuration

---

### `flowgate models`

Display available models for each adapter.

```bash
flowgate models [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `~/.flowgate/config.yaml` | Path to config file |

**Output includes:**

- Models grouped by adapter
- Model aliases (fast, quality, thinking, etc.)
- Provider-specific model identifiers

---

## Global Flags

These flags are available on all commands:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `~/.flowgate/config.yaml` | Path to configuration file |

---

## Configuration Files

### `~/.flowgate/config.yaml`

Main configuration file:

```yaml
default_adapter: anthropic
default_model: claude-sonnet-4-20250514

# Budget limits (optional)
budget:
  max_cost_per_run: 10.00
  warn_threshold: 5.00
```

### `~/.flowgate/routing.yaml`

Task routing configuration (see Routing Configuration section).

### `configs/models.yaml`

Model alias definitions:

```yaml
aliases:
  fast: claude-sonnet-4-20250514
  quality: claude-opus-4-20250514
  thinking: gpt-5.2-thinking
  cheap: deepseek-chat
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Gate failure |
| 4 | Attestation verification failed |
