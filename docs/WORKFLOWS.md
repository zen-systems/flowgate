# Flowgate Workflows Guide

This guide covers common workflows and usage patterns for Flowgate.

## Quick Start

### 1. Set Up API Keys

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export GOOGLE_API_KEY="..."
export DEEPSEEK_API_KEY="..."
```

### 2. Single-Shot Prompts

```bash
# Basic query with automatic routing
flowgate ask "How do I implement a binary tree in Go?"

# Force a specific model
flowgate ask --model claude-opus-4-20250514 "Review this architecture"

# Use a fast model for quick questions
flowgate ask --model fast "What's the syntax for Go interfaces?"
```

### 3. Pipeline Execution

```bash
# Run a feature implementation pipeline
flowgate run --input "Add user authentication" pipelines/feature.yaml

# Allow shell commands in gates
flowgate run --yes --input "Add unit tests" pipelines/feature.yaml
```

---

## Workflow: Feature Implementation

### Step 1: Create Pipeline Manifest

`pipelines/feature.yaml`:

```yaml
name: feature-implementation
description: Implement a new feature with research, planning, and testing

default_adapter: anthropic
default_model: claude-sonnet-4-20250514

gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

  lint:
    type: command
    command: ["golangci-lint", "run"]

stages:
  - name: research
    task_type: research
    prompt: |
      Research the best approach to implement:
      {{ .Input }}

      Consider:
      1. Existing patterns in the codebase
      2. Best practices
      3. Potential edge cases

  - name: outline
    task_type: outline
    prompt: |
      Based on this research:
      {{ .Artifacts.research.Text }}

      Create a detailed implementation plan:
      1. Files to create/modify
      2. Functions and types needed
      3. Test cases to write

  - name: implement
    task_type: implement
    model: claude-opus-4-20250514
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
```

### Step 2: Run the Pipeline

```bash
flowgate run --input "Add rate limiting to API endpoints" pipelines/feature.yaml
```

### Step 3: Review Evidence

```bash
# List runs
ls ~/.flowgate/runs/

# View run details
cat ~/.flowgate/runs/<run-id>/run.json | jq .

# View stage output
cat ~/.flowgate/runs/<run-id>/stages/implement.json | jq .
```

### Step 4: Generate Attestation

```bash
flowgate attest <run-id> --output attestation.json
```

---

## Workflow: Code Refactoring

### Pipeline: `pipelines/refactor.yaml`

```yaml
name: refactor-pipeline
description: Refactor code with analysis and verification

gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

  quality:
    type: hollowcheck
    contract: .hollowcheck.yaml

stages:
  - name: analyze
    task_type: research
    prompt: |
      Analyze this code for refactoring opportunities:
      {{ .Input }}

      Focus on:
      - Code duplication
      - Complex functions
      - Poor naming
      - Missing abstractions

  - name: plan
    task_type: outline
    prompt: |
      Based on this analysis:
      {{ .Artifacts.analyze.Text }}

      Create a refactoring plan that:
      1. Maintains all existing functionality
      2. Improves code quality
      3. Minimizes risk

  - name: refactor
    task_type: refactor
    prompt: |
      Execute this refactoring plan:
      {{ .Artifacts.plan.Text }}

      Ensure all tests continue to pass.
    gates: [go_test, quality]
    max_retries: 3
    apply: true

  - name: verify
    task_type: review
    prompt: |
      Verify the refactoring:
      {{ .Artifacts.refactor.Text }}

      Confirm:
      1. No functionality was lost
      2. Code quality improved
      3. No new issues introduced
```

### Usage

```bash
# Refactor a specific file
flowgate run --input "Refactor pkg/handler.go to reduce complexity" pipelines/refactor.yaml

# Refactor with workspace specification
flowgate run --workspace ./myproject --input "Simplify the authentication module" pipelines/refactor.yaml
```

---

## Workflow: Bug Investigation

### Single-Shot Debug

```bash
# Quick debug question
flowgate ask "Why might this Go code panic: map[string]int{} concurrent read/write"

# Detailed investigation
flowgate ask --model thinking "Debug this error trace: [paste error]"
```

### Pipeline: `pipelines/debug.yaml`

```yaml
name: debug-pipeline
description: Investigate and fix bugs

gates:
  go_test:
    type: command
    command: ["go", "test", "-v", "./..."]

stages:
  - name: investigate
    task_type: debug
    prompt: |
      Investigate this bug:
      {{ .Input }}

      Analyze:
      1. Root cause
      2. Affected code paths
      3. Reproduction steps

  - name: fix
    task_type: implement
    prompt: |
      Based on this investigation:
      {{ .Artifacts.investigate.Text }}

      Implement a fix that:
      1. Addresses the root cause
      2. Doesn't introduce regressions
      3. Includes a test case for the bug
    gates: [go_test]
    max_retries: 2
    apply: true
```

---

## Workflow: Code Review

### Single-Shot Review

```bash
# Quick security review
flowgate ask --model quality "Review this code for security issues: [paste code]"

# Architecture review
flowgate ask --model claude-opus-4-20250514 "Review this architecture diagram: [description]"
```

### With Gates

```bash
# Review with quality gates
flowgate ask --gates quality "Review: [paste code]"
```

---

## Workflow: Documentation

### Pipeline: `pipelines/docs.yaml`

```yaml
name: documentation
description: Generate documentation for code

stages:
  - name: analyze
    task_type: research
    prompt: |
      Analyze this codebase structure:
      {{ .Input }}

      Identify:
      1. Main components
      2. Public APIs
      3. Key workflows

  - name: document
    task_type: implement
    prompt: |
      Based on this analysis:
      {{ .Artifacts.analyze.Text }}

      Generate comprehensive documentation including:
      1. Overview
      2. API reference
      3. Usage examples
      4. Configuration guide
    apply: true
```

---

## Workflow: Test Generation

### Pipeline: `pipelines/tests.yaml`

```yaml
name: test-generation
description: Generate tests for existing code

gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

stages:
  - name: analyze
    prompt: |
      Analyze this code for test coverage:
      {{ .Input }}

      Identify:
      1. Untested functions
      2. Edge cases
      3. Error conditions

  - name: generate
    prompt: |
      Based on this analysis:
      {{ .Artifacts.analyze.Text }}

      Generate comprehensive tests covering:
      1. Happy path scenarios
      2. Edge cases
      3. Error handling
      4. Boundary conditions
    gates: [go_test]
    max_retries: 2
    apply: true
```

---

## Advanced Workflows

### Chained Pipelines

Run multiple pipelines in sequence:

```bash
# Research phase
flowgate run --input "User authentication requirements" pipelines/research.yaml

# Get the research output
RESEARCH=$(cat ~/.flowgate/runs/<run-id>/blobs/output-*.txt)

# Implementation phase with research context
flowgate run --input "$RESEARCH" pipelines/implement.yaml
```

### Parallel Model Comparison

Compare outputs from different models:

```bash
# Run same prompt through different models
flowgate ask --model claude-opus-4-20250514 "Implement quicksort in Go" > opus.txt
flowgate ask --model gpt-5.2-codex "Implement quicksort in Go" > gpt.txt
flowgate ask --model deepseek-coder "Implement quicksort in Go" > deepseek.txt

# Compare results
diff opus.txt gpt.txt
```

### CI/CD Integration

```yaml
# .github/workflows/ai-feature.yml
name: AI Feature Implementation
on:
  workflow_dispatch:
    inputs:
      feature:
        description: 'Feature to implement'
        required: true

jobs:
  implement:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install Flowgate
        run: go install github.com/yourorg/flowgate/cmd/flowgate@latest

      - name: Run Pipeline
        run: flowgate run --yes --input "${{ github.event.inputs.feature }}" pipelines/feature.yaml
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}

      - name: Generate Attestation
        run: |
          RUN_ID=$(ls -t ~/.flowgate/runs | head -1)
          flowgate attest $RUN_ID --output attestation.json

      - name: Create PR
        uses: peter-evans/create-pull-request@v5
        with:
          title: "AI: ${{ github.event.inputs.feature }}"
          body: |
            Implemented via Flowgate pipeline.

            See attached attestation for evidence bundle.
          branch: ai-feature-${{ github.run_id }}
```

---

## Tips and Best Practices

### 1. Start Simple

Begin with single-shot prompts, then graduate to pipelines:

```bash
# First, test your prompt
flowgate ask "Your prompt here"

# Then, add gates
flowgate ask --gates go_test "Your prompt here"

# Finally, create a pipeline
```

### 2. Use Appropriate Models

```bash
# Fast model for quick questions
flowgate ask --model fast "Quick syntax question"

# Quality model for important work
flowgate ask --model quality "Security review"

# Thinking model for complex reasoning
flowgate ask --model thinking "Design a distributed system"
```

### 3. Iterate on Pipelines

```bash
# Validate before running
flowgate validate pipelines/feature.yaml

# Dry run to see what would happen
flowgate run --dry-run --input "Test" pipelines/feature.yaml

# Full run
flowgate run --input "Real task" pipelines/feature.yaml
```

### 4. Review Evidence

Always check evidence bundles for important runs:

```bash
# Quick summary
jq '.status, .cost_report.total_cost' ~/.flowgate/runs/<run-id>/run.json

# Gate failures
jq '.gate_results[] | select(.passed == false)' ~/.flowgate/runs/<run-id>/stages/*.json
```

### 5. Generate Attestations

For any production-affecting changes, generate and store attestations:

```bash
flowgate attest <run-id> --output attestations/$(date +%Y%m%d)-<feature>.json
```
