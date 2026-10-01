# Evidence Bundles & Attestation

Flowgate generates comprehensive evidence bundles for every pipeline run, enabling auditable AI workflows and supply chain verification.

## Evidence Bundle Structure

Every pipeline run creates an evidence directory:

```
~/.flowgate/runs/<run-id>/
├── run.json                    # Run-level metadata
├── stages/
│   ├── research.json          # Per-stage records
│   ├── implement.json
│   └── review.json
├── gates/
│   ├── implement-go_test.log  # Gate execution logs
│   └── review-quality.log
└── blobs/
    ├── prompt-<sha256>.txt    # Immutable prompt content
    ├── output-<sha256>.txt    # Immutable output content
    └── repair-<sha256>.txt    # Repair attempt content
```

---

## Run Record (`run.json`)

Top-level metadata for the entire pipeline run:

```json
{
  "id": "abc123def456",
  "timestamp": "2026-01-21T10:30:00Z",
  "pipeline_file": "pipelines/feature.yaml",
  "pipeline_name": "feature-implementation",
  "input_hash": "sha256:e3b0c44298fc1c...",
  "workspace": "/path/to/workspace",
  "tool_versions": {
    "flowgate": "1.0.0",
    "go": "1.22.0",
    "hollowcheck": "0.5.0"
  },
  "cost_report": {
    "total_cost": 0.0234,
    "calls": [
      {
        "adapter": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "input_tokens": 1500,
        "output_tokens": 2000,
        "cost": 0.0117
      }
    ],
    "budget_status": "within_budget"
  },
  "routing_decision": {
    "task_type": "implement",
    "confidence": 0.85,
    "method": "heuristic"
  },
  "status": "passed",
  "duration_ms": 45000
}
```

---

## Stage Record (`stages/<name>.json`)

Detailed record for each pipeline stage:

```json
{
  "name": "implement",
  "adapter": "anthropic",
  "model": "claude-sonnet-4-20250514",
  "prompt_ref": "blobs/prompt-sha256abc.txt",
  "prompt_hash": "sha256:abc123...",
  "output_ref": "blobs/output-sha256def.txt",
  "output_hash": "sha256:def456...",
  "gate_results": [
    {
      "name": "go_test",
      "passed": true,
      "score": 100,
      "violations": [],
      "repair_hints": [],
      "kind": "command",
      "duration_ms": 2500
    },
    {
      "name": "quality",
      "passed": true,
      "score": 95,
      "violations": [],
      "kind": "hollowcheck"
    }
  ],
  "attempts": [
    {
      "attempt": 1,
      "output_hash": "sha256:def456...",
      "gate_results": [
        {"name": "go_test", "passed": true}
      ]
    }
  ],
  "apply_result": {
    "applied": true,
    "files_affected": [
      "pkg/handler.go",
      "pkg/handler_test.go"
    ],
    "diff_ref": "blobs/diff-sha256xyz.txt"
  },
  "duration_ms": 15000
}
```

---

## Gate Logs (`gates/<stage>-<gate>.log`)

Raw output from gate execution:

```
=== Gate: go_test ===
Command: go test ./...
Working Directory: /path/to/workspace
Started: 2026-01-21T10:30:15Z

ok      myproject/pkg/handler   0.234s
ok      myproject/pkg/service   0.456s

Exit Code: 0
Duration: 2.5s
Passed: true
```

---

## Blob Storage

All prompts and outputs are stored as immutable blobs with content-addressable hashing:

```
blobs/prompt-e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.txt
```

- **SHA-256 hash** ensures integrity
- **Immutable**: Once written, never modified
- **Deduplicated**: Identical content shares the same blob

---

## Attestation Generation

Generate a cryptographic attestation from a completed run:

```bash
flowgate attest <run-id>
```

### Attestation Schema (v0)

```json
{
  "schema": "flowgate.attestation.v0",
  "subject": {
    "workspace": "/path/to/workspace",
    "pipeline_file": "pipelines/feature.yaml",
    "run_id": "abc123def456",
    "stage": "implement"
  },
  "claim": {
    "passed": true,
    "gate_count": 2,
    "gated": true,
    "gates": [
      {
        "name": "go_test",
        "passed": true,
        "score": 100,
        "violations": []
      },
      {
        "name": "quality",
        "passed": true,
        "score": 95,
        "violations": []
      }
    ]
  },
  "evidence": {
    "run_json": "<base64 encoded run.json>",
    "stage_json": "<base64 encoded stage.json>",
    "blobs": {
      "prompt-sha256abc.txt": "<base64 content>",
      "output-sha256def.txt": "<base64 content>"
    },
    "gate_logs": {
      "go_test": "<base64 log content>",
      "quality": "<base64 log content>"
    }
  },
  "hashes": {
    "run.json": "sha256:...",
    "stages/implement.json": "sha256:...",
    "blobs/prompt-sha256abc.txt": "sha256:abc123...",
    "blobs/output-sha256def.txt": "sha256:def456...",
    "gates/implement-go_test.log": "sha256:..."
  },
  "generated_at": "2026-01-21T10:35:00Z"
}
```

---

## Attestation Verification

Verify attestation integrity:

```bash
flowgate verify attestation.json
```

### Verification Checks

1. **Hash Verification**: All hashes in `hashes` map match SHA-256 of actual content
2. **Claim Consistency**: `claim.gates` matches recorded stage gate results exactly
3. **Evidence Completeness**: All referenced blobs and logs are present
4. **Schema Compatibility**: Attestation schema version is supported

### Verification Output

```
Verifying attestation for run abc123def456...

✓ Hash verification passed (5/5 artifacts)
✓ Claim consistency verified
✓ Evidence bundle complete
✓ Schema version supported (v0)

Attestation VALID
```

### Verification Failures

```
Verifying attestation for run abc123def456...

✗ Hash mismatch: blobs/output-sha256def.txt
  Expected: sha256:def456...
  Actual:   sha256:xyz789...

✗ Claim mismatch: gate "go_test" claims passed=true but stage record shows passed=false

Attestation INVALID
```

---

## Use Cases

### 1. Audit Trail

Every AI interaction is recorded with:
- Exact prompts sent
- Exact outputs received
- Gate evaluation results
- Timing and cost information

```bash
# View a past run
cat ~/.flowgate/runs/<run-id>/run.json | jq .

# View specific stage
cat ~/.flowgate/runs/<run-id>/stages/implement.json | jq .
```

### 2. Supply Chain Verification

Verify that code was generated through a gated pipeline:

```bash
# Generate attestation after successful run
flowgate attest <run-id> --output attestation.json

# Later, verify the attestation
flowgate verify attestation.json
```

### 3. Reproducibility

Evidence bundles capture everything needed to understand a run:
- Input and configuration
- Model versions used
- Gate results and failures
- Repair attempts

### 4. Compliance

For regulated environments:
- Immutable evidence with cryptographic hashes
- Complete audit trail
- Verifiable claims about quality gate passage

---

## Evidence Retention

By default, evidence is stored in `~/.flowgate/runs/`. Consider:

1. **Backup**: Regularly backup the runs directory
2. **Archival**: Move old runs to long-term storage
3. **Cleanup**: Remove runs that are no longer needed

```bash
# List all runs
ls ~/.flowgate/runs/

# Archive old runs
tar -czf flowgate-runs-backup.tar.gz ~/.flowgate/runs/

# Clean up runs older than 30 days
find ~/.flowgate/runs -maxdepth 1 -type d -mtime +30 -exec rm -rf {} \;
```

---

## Integrating with CI/CD

### Generate Attestation in CI

```yaml
# .github/workflows/ai-workflow.yml
jobs:
  implement:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run AI Pipeline
        run: flowgate run --input "${{ github.event.inputs.feature }}" pipelines/feature.yaml
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}

      - name: Generate Attestation
        run: |
          RUN_ID=$(ls -t ~/.flowgate/runs | head -1)
          flowgate attest $RUN_ID --output attestation.json

      - name: Upload Attestation
        uses: actions/upload-artifact@v4
        with:
          name: attestation
          path: attestation.json
```

### Verify Before Deploy

```yaml
  deploy:
    needs: implement
    runs-on: ubuntu-latest
    steps:
      - name: Download Attestation
        uses: actions/download-artifact@v4
        with:
          name: attestation

      - name: Verify Attestation
        run: flowgate verify attestation.json

      - name: Deploy
        if: success()
        run: ./deploy.sh
```
