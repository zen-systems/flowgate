# Pipeline Manifest Specification

Flowgate pipelines are defined in YAML manifests that specify multi-stage AI workflows with quality gates, repair loops, and evidence collection.

## Basic Structure

```yaml
name: my-pipeline
description: A description of what this pipeline does

# Optional defaults
default_adapter: anthropic
default_model: claude-sonnet-4-20250514

# Optional workspace configuration
workspace:
  path: ./output

# Gate definitions (reusable across stages)
gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

  quality:
    type: hollowcheck
    contract: .hollowcheck.yaml

# Pipeline stages
stages:
  - name: analyze
    prompt: |
      Analyze the following requirement:
      {{ .Input }}

  - name: implement
    prompt: |
      Based on this analysis:
      {{ .Artifacts.analyze.Text }}

      Implement the solution.
    gates: [go_test, quality]
    max_retries: 2
```

---

## Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique pipeline identifier |
| `description` | string | No | Human-readable description |
| `default_adapter` | string | No | Default LLM adapter for all stages |
| `default_model` | string | No | Default model for all stages |
| `workspace` | object | No | Workspace configuration |
| `gates` | map | No | Reusable gate definitions |
| `stages` | array | Yes | Ordered list of pipeline stages |

---

## Workspace Configuration

```yaml
workspace:
  path: ./output          # Working directory for file operations
  clone_from: ./source    # Optional: clone from existing directory
```

---

## Gate Definitions

Gates are quality checks that run after stage completion. Define them once, reference them in stages.

### Command Gates

Execute shell commands and check exit codes:

```yaml
gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

  lint:
    type: command
    command: ["golangci-lint", "run"]

  build:
    type: command
    command: ["go", "build", "-o", "app", "."]
```

**Command Placeholders:**

| Placeholder | Description |
|-------------|-------------|
| `{path}` | Workspace-confined file path |
| `{pkg}` | Go package path |

### Hollowcheck Gates

Run code quality analysis with hollowcheck:

```yaml
gates:
  quality:
    type: hollowcheck
    contract: .hollowcheck.yaml    # Path to quality contract
```

---

## Stage Configuration

Each stage represents a single LLM interaction with optional quality gates.

```yaml
stages:
  - name: implement           # Required: unique stage name
    task_type: implement      # Optional: for routing
    adapter: anthropic        # Optional: override default
    model: claude-opus-4-20250514  # Optional: override default
    fallback_model: claude-sonnet-4-20250514  # Optional: fallback on errors

    prompt: |                 # Required: prompt template
      Implement this feature:
      {{ .Input }}

    gates: [go_test, lint]    # Optional: gates to run after completion
    max_retries: 3            # Optional: repair attempts on gate failure

    apply: true               # Optional: apply file changes to workspace
    escalate_on: security     # Optional: escalate to human on specific issues
```

### Stage Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | Required | Unique stage identifier |
| `task_type` | string | | Task type for routing (research, implement, etc.) |
| `adapter` | string | `default_adapter` | LLM adapter to use |
| `model` | string | `default_model` | Model to use |
| `fallback_model` | string | | Fallback model on primary failure |
| `prompt` | string | Required | Prompt template with variables |
| `gates` | array | `[]` | Gate names to run after completion |
| `max_retries` | int | `0` | Repair loop attempts on gate failure |
| `apply` | bool | `false` | Apply file changes to workspace |
| `escalate_on` | string | | Escalate to human on specific condition |

---

## Prompt Templating

Prompts support Go template syntax with these variables:

### Available Variables

| Variable | Description |
|----------|-------------|
| `{{ .Input }}` | Original pipeline input |
| `{{ .Artifacts.<stage>.Text }}` | Text output from a previous stage |
| `{{ .Artifacts.<stage>.Hash }}` | SHA-256 hash of stage output |
| `{{ .Workspace }}` | Workspace path |
| `{{ .RunID }}` | Current run identifier |

### Example Templates

```yaml
stages:
  - name: research
    prompt: |
      Research the following topic:
      {{ .Input }}

      Provide a comprehensive analysis.

  - name: outline
    prompt: |
      Based on this research:
      {{ .Artifacts.research.Text }}

      Create a detailed implementation outline.

  - name: implement
    prompt: |
      Following this outline:
      {{ .Artifacts.outline.Text }}

      Implement the solution in the workspace at {{ .Workspace }}.
```

---

## Repair Loops

When gates fail, flowgate can automatically attempt repairs:

```yaml
stages:
  - name: implement
    prompt: |
      Implement {{ .Input }}
    gates: [go_test]
    max_retries: 3
```

**Repair Loop Process:**

1. Stage executes and produces output
2. Gates evaluate the output
3. If gates fail:
   - Gate violations and repair hints are collected
   - A repair prompt is generated with failure context
   - Stage re-executes (up to `max_retries` times)
4. If all retries fail, pipeline fails

**Repair Prompt Injection:**

```
Previous attempt failed with these violations:
- test "TestFoo" failed: expected 5, got 3

Repair hints:
- Check the calculation logic in calculateTotal()

Please fix the issues and try again.
```

---

## Apply Operations

Stages with `apply: true` can modify the workspace:

```yaml
stages:
  - name: implement
    prompt: |
      Create a new file at pkg/handler.go with...
    apply: true    # Changes will be applied to workspace
```

**Apply Behavior:**

- By default, apply operations run in dry-run mode
- Use `--yes` flag to actually apply changes
- Changes are tracked in evidence bundle
- File diffs are recorded

---

## Complete Example

```yaml
name: feature-implementation
description: Implement a new feature with research, planning, and testing

default_adapter: anthropic
default_model: claude-sonnet-4-20250514

workspace:
  path: .

gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

  lint:
    type: command
    command: ["golangci-lint", "run"]

  quality:
    type: hollowcheck
    contract: .hollowcheck.yaml

stages:
  - name: research
    task_type: research
    prompt: |
      Research the best approach to implement:
      {{ .Input }}

      Consider existing patterns in the codebase.

  - name: outline
    task_type: outline
    prompt: |
      Based on this research:
      {{ .Artifacts.research.Text }}

      Create a detailed implementation plan with:
      1. Files to create/modify
      2. Functions and types needed
      3. Test cases to write

  - name: implement
    task_type: implement
    model: claude-opus-4-20250514  # Use stronger model for implementation
    prompt: |
      Following this plan:
      {{ .Artifacts.outline.Text }}

      Implement the feature. Output complete file contents.
    gates: [go_test, lint]
    max_retries: 2
    apply: true

  - name: review
    task_type: review
    prompt: |
      Review this implementation:
      {{ .Artifacts.implement.Text }}

      Check for:
      - Security issues
      - Performance concerns
      - Code style consistency
    gates: [quality]
```

---

## Task Types for Routing

When `task_type` is specified, flowgate can automatically select the optimal model:

| Task Type | Best For |
|-----------|----------|
| `research` | Information gathering, analysis |
| `summarize` | Condensing information |
| `outline` | Planning, structuring |
| `scaffold` | Creating project structure |
| `implement` | Writing code |
| `refactor` | Improving existing code |
| `debug` | Finding and fixing bugs |
| `review` | Code review, security analysis |
| `math` | Mathematical reasoning |
| `reasoning` | Complex logical problems |

---

## Validation

Validate manifests before running:

```bash
flowgate validate pipeline.yaml
```

**Validation Rules:**

- `name` is required
- `stages` must be non-empty
- Stage names must be unique
- Referenced gates must be defined
- Template syntax must be valid
