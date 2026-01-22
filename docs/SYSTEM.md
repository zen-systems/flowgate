# Flowgate System Overview

This document describes Flowgate capabilities, CLI usage/flags, and the pipeline spec.

## Summary

Flowgate is a local-first AI orchestration system with:
- Multi-stage pipelines driven by YAML manifests.
- Adapter selection and prompt templating.
- Plural gates (command, hollowcheck) with repair loops.
- Evidence bundles per run (JSON + logs + blobs).
- Attestation generation and verification.
- Dry-run-by-default workspace applies with explicit approval.
- Command execution policy (capabilities/templates, deny-shell).

## Core Capabilities

### Pipeline Execution
- Load a pipeline manifest from YAML.
- Execute stages in order with prompt templating.
- Stage outputs are available to later stages.
- Gates run in order; failure triggers repair loops up to `max_retries`.
- Fail-closed: gate errors/failures stop the stage unless repaired.

### Adapters
- Built-in adapters for Anthropic/OpenAI/Google/DeepSeek (API keys via env vars only).
- `mock` adapter for deterministic local runs and tests.

### Smart Routing
- Heuristic trigger matching with confidence scoring (fast path).
- Optional LLM tie-breaker using a separate classifier adapter/model.
- Routing decision recorded in evidence with candidates, reasons, and post-run feedback.

### Gates
- `command` gate: run a local command and capture stdout/stderr/exit code.
- `hollowcheck` gate: text/code quality gate via hollowcheck CLI.

Command gate policy:
- Deny shell by default (`sh -c`, `bash -c`, `zsh -c` blocked).
- Allowlist via capabilities or templates; legacy `allowed_commands` supported.
- Placeholders: `{path}` must be workspace-confined; `{pkg}` is restricted.
- If `deny_shell: false`, running shell commands requires `--yes` at runtime.

### Workspace Apply
- `apply: true` stages use dry-run-by-default: apply output to a temp clone and gate there.
- `--apply --yes` required to modify the real workspace.

### Evidence Bundles
- Stored in `.flowgate/runs/<run-id>/` by default (0700/0600 permissions).
- Run JSON + per-stage JSON + gate logs + blobs for full prompt/output.
- Attempt-level evidence includes prompt/output refs and workspace used.

### Attestations
- `flowgate attest` creates a v0 attestation JSON referencing evidence + hashes.
- `flowgate verify` validates hashes and claim consistency.

## CLI Usage

### `flowgate run`
Execute a pipeline.

```bash
flowgate run -f pipelines/examples/feature.yaml -i "input"
```

Flags:
- `-f, --file` (required): path to pipeline manifest
- `-i, --input`: input string (if omitted, read stdin)
- `--workspace`: workspace path (defaults to cwd)
- `--out`: evidence base directory
- `--apply`: apply changes to the real workspace
- `--yes`: approve applying changes and allow shell if `deny_shell: false`

Notes:
- `--apply` requires `--yes` or the run fails before touching the workspace.

### `flowgate ask`
Single-shot prompt with routing and optional gates.

```bash
flowgate ask "Implement a rate limiter" --gate hollowcheck.yaml
```

Flags:
- `--adapter`, `--model`: override routing
- `--deep`: enable curator
- `--gate`: hollowcheck contract path
- `--retries`: max repair attempts

### `flowgate validate`
Validate a pipeline manifest.

```bash
flowgate validate pipelines/examples/feature.yaml
```

### `flowgate attest`
Generate a v0 attestation for a stage.

```bash
flowgate attest --run .flowgate/runs/<run-id> --stage implement --out /tmp/att.json
```

### `flowgate verify`
Verify an attestation against a run directory.

```bash
flowgate verify --attestation /tmp/att.json --run .flowgate/runs/<run-id>
```

### `flowgate routes` / `flowgate models`
Show routing rules and available models.

## Configuration

### Environment Variables (API keys only)
- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY`
- `GOOGLE_API_KEY`
- `DEEPSEEK_API_KEY`

Config file API keys are ignored for security.

### Routing Config
`~/.flowgate/routing.yaml` (see `configs/routing.yaml` for example).

Classifier fields:
- `classifier_adapter` / `classifier_model`: adapter/model for LLM tie-breaker
- `classifier_confidence_threshold`: default `0.65`
- `enable_llm_tie_breaker`: default `true`

## Manifest Specification (v1)

### Top-level
```yaml
name: string
description: string
workspace:
  path: string

# Optional global gates
# These are referenced by name in stages.gates
gates:
  <gate_name>:
    type: command | hollowcheck
    # command gate fields
    command: ["go", "test", "./..."]
    workdir: .
    deny_shell: true
    capability: go_test | go_vet | gofmt
    templates:
      - exec: go
        args: ["test", "./..."]
    allowed_commands: ["legacy exact command"]
    # hollowcheck gate fields
    binary_path: path
    contract_path: path

stages:
  - name: string
    task_type: string
    adapter: string
    model: string
    fallback_model: string
    prompt: |
      {{ .Input }}
    apply: bool
    gates: [gate_name]
    max_retries: int
```

### Stage Prompt Templating
Available variables:
- `.Input` / `.input`: pipeline input
- `.Artifacts.<stageName>.Text` or `.Artifacts.<stageName>.Output`: output text
- `.Stages.<stageName>.output` (legacy compatibility)

### Command Gate Policy
- Policy order:
  1) `deny_shell` (default true)
  2) capability templates, else templates, else legacy `allowed_commands`
  3) match must be exact, including arg count
- Placeholders:
  - `{path}`: confined to workspace
  - `{pkg}`: one of `./...`, `./pkg/...`, `./cmd/...`

### Workspace Apply
- `apply: true` stage outputs are treated as patches (unified diff preferred; file blocks supported).
- Dry-run by default: apply + gates on temp clone.
- `--apply --yes` required for real workspace writes.

## Evidence Bundle Layout
```
.flowgate/runs/<run-id>/
  run.json
  stages/<stage>.json
  gates/<stage>-<gate>.log
  blobs/<kind>-<sha>.txt
```

Evidence fields of note:
- `stage.prompt`/`stage.output`: truncated previews
- `stage.prompt_ref`/`stage.output_ref`: blob refs
- `attempts[].prompt_ref`/`output_ref`: per-attempt blob refs
- `attempts[].workspace_mode`: "temp" or "real"
- `routing_decision`: task_type, confidence, candidates, and post-run feedback

## Attestation (v0)
Attestation structure:
```json
{
  "schema": "flowgate.attestation.v0",
  "subject": {
    "workspace": "...",
    "pipeline_file": "...",
    "run_id": "...",
    "stage": "..."
  },
  "claim": {
    "passed": true,
    "gate_count": 1,
    "gated": true,
    "gates": [
      {"name": "go_test", "kind": "command", "passed": true, "score": 0}
    ]
  },
  "evidence": {
    "run_json": "run.json",
    "stage_json": "stages/implement.json",
    "blobs": ["blobs/prompt-...txt", "blobs/output-...txt"],
    "gate_logs": ["gates/implement-go_test.log"]
  },
  "hashes": {
    "run.json": "...",
    "stages/implement.json": "...",
    "blobs/prompt-...txt": "..."
  }
}
```

Verification checks:
- All hash references must exist and match SHA-256.
- Claim gates must match stage gate results exactly.
- Legacy attestations (no schema) are accepted with legacy claim semantics.

## Security Defaults
- Evidence directories 0700, files 0600.
- API keys only from env.
- Shell execution denied by default; explicit approval required when `deny_shell: false`.
- Workspace apply dry-run by default.
