# Quality Gates

Flowgate enforces code quality through gates - automated checks that validate AI-generated outputs before they're accepted.

## Gate Types

### Command Gates

Execute shell commands and evaluate based on exit codes:

```yaml
gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

  lint:
    type: command
    command: ["golangci-lint", "run"]

  npm_test:
    type: command
    command: ["npm", "test"]
```

### Hollowcheck Gates

Run code quality analysis using hollowcheck contracts:

```yaml
gates:
  quality:
    type: hollowcheck
    contract: .hollowcheck.yaml
```

---

## Command Gate Security

Command gates have a security-first design with shell execution **denied by default**.

### Default Policy

```yaml
# These are BLOCKED by default:
command: ["sh", "-c", "echo hello"]     # Shell execution
command: ["bash", "-c", "rm -rf /"]     # Shell execution
command: ["/bin/sh", "script.sh"]       # Shell execution
```

### Allowing Commands

Commands must be explicitly allowed via:

1. **Exact command match**: Full command including arguments
2. **Template matching**: Command patterns with placeholders
3. **`--yes` flag**: Runtime permission for shell commands

### Command Templates

Use placeholders for workspace-safe commands:

| Placeholder | Description | Security |
|-------------|-------------|----------|
| `{path}` | File path within workspace | Confined to workspace |
| `{pkg}` | Go package path | Validated format |

```yaml
gates:
  test_file:
    type: command
    command: ["go", "test", "{path}"]    # {path} is workspace-confined

  lint_pkg:
    type: command
    command: ["golangci-lint", "run", "{pkg}"]
```

### Shell Execution with `--yes`

To run commands that require shell interpretation:

```bash
# Without --yes: shell commands are blocked
flowgate run pipelines/feature.yaml
# Error: shell execution denied for gate "custom_script"

# With --yes: shell commands are allowed
flowgate run --yes pipelines/feature.yaml
# Gate "custom_script" executed successfully
```

---

## Gate Results

Gates return structured results:

```json
{
  "name": "go_test",
  "passed": true,
  "score": 100,
  "violations": [],
  "repair_hints": [],
  "kind": "command",
  "diagnostics": {
    "exit_code": 0,
    "stdout": "ok myproject/pkg/handler 0.234s",
    "stderr": "",
    "duration_ms": 2500
  }
}
```

### Result Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Gate identifier |
| `passed` | bool | Whether the gate passed |
| `score` | int | Quality score (0-100) |
| `violations` | array | List of specific failures |
| `repair_hints` | array | Suggestions for fixing failures |
| `kind` | string | Gate type: `command` or `hollowcheck` |
| `diagnostics` | object | Gate-specific diagnostic data |

---

## Violations and Repair Hints

When gates fail, they provide actionable information:

### Command Gate Failure

```json
{
  "name": "go_test",
  "passed": false,
  "violations": [
    {
      "type": "test_failure",
      "message": "TestCalculateTotal failed",
      "location": "pkg/handler_test.go:45",
      "details": "expected 100, got 95"
    }
  ],
  "repair_hints": [
    "Check the calculation logic in calculateTotal()",
    "Verify the discount percentage is applied correctly"
  ]
}
```

### Hollowcheck Gate Failure

```json
{
  "name": "quality",
  "passed": false,
  "score": 65,
  "violations": [
    {
      "rule": "stub_function",
      "message": "Function handleError appears to be a stub",
      "file": "pkg/handler.go",
      "line": 45,
      "severity": "warning"
    },
    {
      "rule": "missing_test",
      "message": "Function processOrder has no test coverage",
      "file": "pkg/service.go",
      "line": 120,
      "severity": "error"
    }
  ],
  "repair_hints": [
    "Implement the handleError function body",
    "Add test case for processOrder function"
  ]
}
```

---

## Repair Loops

Gates integrate with repair loops to automatically fix failures:

```yaml
stages:
  - name: implement
    prompt: "Implement {{ .Input }}"
    gates: [go_test, lint]
    max_retries: 3
```

### Repair Process

1. **Initial Execution**: Stage runs with original prompt
2. **Gate Evaluation**: All gates are evaluated
3. **Failure Detection**: If any gate fails:
   - Violations are collected
   - Repair hints are gathered
   - A repair prompt is constructed
4. **Repair Attempt**: Stage re-runs with repair context
5. **Repeat**: Until gates pass or `max_retries` exhausted

### Repair Prompt

```
Your previous implementation had the following issues:

Gate "go_test" failed:
- TestCalculateTotal failed: expected 100, got 95

Gate "lint" failed:
- pkg/handler.go:30: exported function Handler should have comment

Repair hints:
- Check the calculation logic in calculateTotal()
- Add documentation comment to exported Handler function

Please fix these issues and provide an updated implementation.
```

---

## Using Gates in Pipelines

### Basic Gate Usage

```yaml
stages:
  - name: implement
    prompt: "Implement {{ .Input }}"
    gates: [go_test]
```

### Multiple Gates

```yaml
stages:
  - name: implement
    prompt: "Implement {{ .Input }}"
    gates: [go_test, lint, security, quality]
```

### Gates with Repair

```yaml
stages:
  - name: implement
    prompt: "Implement {{ .Input }}"
    gates: [go_test, lint]
    max_retries: 3    # Try up to 3 times to fix issues
```

### Different Gates per Stage

```yaml
stages:
  - name: implement
    prompt: "Write the code"
    gates: [compile, lint]

  - name: test
    prompt: "Write tests for {{ .Artifacts.implement.Text }}"
    gates: [go_test]

  - name: review
    prompt: "Review the implementation"
    gates: [quality, security]
```

---

## Gate Definitions

### Full Command Gate Options

```yaml
gates:
  custom_test:
    type: command
    command: ["go", "test", "-v", "-race", "./..."]
    timeout: 300          # Timeout in seconds
    working_dir: "."      # Relative to workspace
    env:                  # Additional environment variables
      CGO_ENABLED: "1"
```

### Full Hollowcheck Gate Options

```yaml
gates:
  quality:
    type: hollowcheck
    contract: .hollowcheck.yaml    # Path to quality contract
    fail_on_warning: false         # Only fail on errors
    min_score: 80                  # Minimum passing score
```

---

## Built-in Gate Patterns

### Go Testing

```yaml
gates:
  go_test:
    type: command
    command: ["go", "test", "./..."]

  go_test_race:
    type: command
    command: ["go", "test", "-race", "./..."]

  go_test_coverage:
    type: command
    command: ["go", "test", "-cover", "./..."]
```

### Go Linting

```yaml
gates:
  golangci_lint:
    type: command
    command: ["golangci-lint", "run"]

  go_vet:
    type: command
    command: ["go", "vet", "./..."]
```

### JavaScript/TypeScript

```yaml
gates:
  npm_test:
    type: command
    command: ["npm", "test"]

  npm_lint:
    type: command
    command: ["npm", "run", "lint"]

  npm_typecheck:
    type: command
    command: ["npm", "run", "typecheck"]
```

### Python

```yaml
gates:
  pytest:
    type: command
    command: ["pytest"]

  ruff:
    type: command
    command: ["ruff", "check", "."]

  mypy:
    type: command
    command: ["mypy", "."]
```

---

## Gate Evidence

All gate executions are recorded in the evidence bundle:

### Gate Log Location

```
~/.flowgate/runs/<run-id>/gates/<stage>-<gate>.log
```

### Gate Log Contents

```
=== Gate: go_test ===
Command: go test ./...
Working Directory: /path/to/workspace
Started: 2026-01-21T10:30:15Z
Environment:
  GOPATH=/go
  CGO_ENABLED=0

--- STDOUT ---
ok      myproject/pkg/handler   0.234s
ok      myproject/pkg/service   0.456s

--- STDERR ---
(empty)

Exit Code: 0
Duration: 2.5s
Result: PASSED
```

---

## Best Practices

### 1. Fast Gates First

Order gates from fastest to slowest to fail fast:

```yaml
gates: [compile, lint, go_test]  # compile is fastest
```

### 2. Appropriate Retry Counts

```yaml
# Simple fixes: 1-2 retries
- name: format
  gates: [lint]
  max_retries: 1

# Complex fixes: 2-3 retries
- name: implement
  gates: [go_test]
  max_retries: 3
```

### 3. Targeted Gates

Use specific gates for specific stages:

```yaml
stages:
  - name: implement
    gates: [compile, lint]      # Quick syntax checks

  - name: test
    gates: [go_test]            # Full test suite

  - name: final
    gates: [quality, security]  # Comprehensive checks
```

### 4. Meaningful Repair Hints

Configure gates to provide actionable hints when possible.
