# Intelligent Routing System

Flowgate automatically routes prompts to the most appropriate LLM based on task type detection. This document explains how routing works and how to configure it.

## How Routing Works

```
┌─────────────────┐
│  User Prompt    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Heuristic Match │──── High confidence ────▶ Route to matched adapter/model
└────────┬────────┘
         │
         │ Low confidence
         ▼
┌─────────────────┐
│ LLM Classifier  │──── Classification ────▶ Route to classified adapter/model
│   (optional)    │
└────────┬────────┘
         │
         │ No match
         ▼
┌─────────────────┐
│ Default Model   │
└─────────────────┘
```

### Routing Steps

1. **Heuristic Matching**: Fast keyword-based analysis of the prompt
2. **Confidence Scoring**: Calculate match confidence (0.0 - 1.0)
3. **LLM Tie-Breaker**: If confidence is below threshold, optionally use an LLM to classify
4. **Fallback**: Use default adapter/model if no match

---

## Task Types

Flowgate recognizes these task types, each mapped to optimal models:

| Task Type | Description | Typical Triggers |
|-----------|-------------|------------------|
| `research` | Information gathering | "research", "investigate", "analyze" |
| `summarize` | Condensing information | "summarize", "tldr", "brief" |
| `outline` | Planning and structuring | "outline", "plan", "structure" |
| `scaffold` | Project setup | "scaffold", "bootstrap", "init" |
| `implement` | Writing new code | "implement", "write", "create" |
| `refactor` | Improving code | "refactor", "clean up", "improve" |
| `debug` | Bug fixing | "debug", "fix", "troubleshoot" |
| `review` | Code review | "review", "check", "audit" |
| `security_review` | Security analysis | "security", "vulnerability", "pentest" |
| `math` | Mathematical problems | "calculate", "prove", "derive" |
| `reasoning` | Complex logic | "reason", "deduce", "analyze" |
| `architecture` | System design | "architect", "design", "structure" |
| `legal` | Legal analysis | "legal", "compliance", "regulation" |
| `complex_debug` | Difficult bugs | complex debugging patterns |
| `final_review` | Final quality check | "final review", "sign off" |
| `bulk_code` | Large code generation | "generate all", "complete implementation" |

---

## Configuration

### Routing Rules (`~/.flowgate/routing.yaml`)

```yaml
# Classifier settings
classifier:
  adapter: anthropic
  model: claude-sonnet-4-20250514
  confidence_threshold: 0.7
  enable_llm_tie_breaker: true

# Default fallback
default:
  adapter: anthropic
  model: claude-sonnet-4-20250514

# Task type mappings
task_types:
  research:
    adapter: google
    model: gemini-3-pro
    triggers:
      - "research"
      - "investigate"
      - "look into"
      - "find out"

  implement:
    adapter: anthropic
    model: claude-sonnet-4-20250514
    triggers:
      - "implement"
      - "write"
      - "create"
      - "build"

  reasoning:
    adapter: openai
    model: gpt-5.2-thinking
    triggers:
      - "reason"
      - "think through"
      - "step by step"

  math:
    adapter: deepseek
    model: deepseek-reasoner
    triggers:
      - "calculate"
      - "compute"
      - "prove"
      - "derive"

  review:
    adapter: anthropic
    model: claude-opus-4-20250514
    triggers:
      - "review"
      - "check"
      - "audit"
      - "evaluate"
```

### Model Aliases (`configs/models.yaml`)

```yaml
aliases:
  # Speed-optimized
  fast: claude-sonnet-4-20250514
  fast-code: deepseek-coder

  # Quality-optimized
  quality: claude-opus-4-20250514
  deep: gpt-5.2-pro

  # Reasoning-optimized
  thinking: gpt-5.2-thinking
  reason: deepseek-reasoner

  # Cost-optimized
  cheap: deepseek-chat

  # Research-optimized
  research: gemini-3-pro
```

---

## Provider Model Reference

### Anthropic

| Model | Best For |
|-------|----------|
| `claude-opus-4-20250514` | Complex reasoning, quality-critical tasks |
| `claude-sonnet-4-20250514` | General purpose, balanced speed/quality |

### OpenAI

| Model | Best For |
|-------|----------|
| `gpt-5.2-instant` | Fast responses |
| `gpt-5.2-thinking` | Complex reasoning, step-by-step |
| `gpt-5.2-codex` | Code generation |
| `gpt-5.2-pro` | Highest quality |

### Google

| Model | Best For |
|-------|----------|
| `gemini-3-pro` | Research, large context |

### DeepSeek

| Model | Best For |
|-------|----------|
| `deepseek-chat` | Cost-effective general use |
| `deepseek-coder` | Code generation |
| `deepseek-reasoner` | Mathematical reasoning |

---

## Routing in Practice

### Viewing Current Routes

```bash
flowgate routes
```

Output:
```
Task Type Routing Configuration
================================

Classifier:
  Adapter: anthropic
  Model: claude-sonnet-4-20250514
  Confidence Threshold: 0.7
  LLM Tie-Breaker: enabled

Default:
  Adapter: anthropic
  Model: claude-sonnet-4-20250514

Task Types:
  research:
    Adapter: google
    Model: gemini-3-pro
    Triggers: research, investigate, look into, find out

  implement:
    Adapter: anthropic
    Model: claude-sonnet-4-20250514
    Triggers: implement, write, create, build
  ...
```

### Viewing Available Models

```bash
flowgate models
```

Output:
```
Available Models by Adapter
============================

anthropic:
  - claude-opus-4-20250514
  - claude-sonnet-4-20250514

openai:
  - gpt-5.2-instant
  - gpt-5.2-thinking
  - gpt-5.2-codex
  - gpt-5.2-pro

google:
  - gemini-3-pro

deepseek:
  - deepseek-chat
  - deepseek-coder
  - deepseek-reasoner

Aliases:
  fast -> claude-sonnet-4-20250514
  quality -> claude-opus-4-20250514
  thinking -> gpt-5.2-thinking
  ...
```

### Override Routing

Force specific adapter/model:

```bash
# Use specific adapter and model
flowgate ask --adapter openai --model gpt-5.2-thinking "Complex reasoning task"

# Use model alias
flowgate ask --model fast "Quick question"

# Use quality model regardless of task type
flowgate ask --model quality "Important review"
```

---

## Routing Decision Evidence

Every routing decision is recorded in the evidence bundle:

```json
{
  "routing_decision": {
    "task_type": "implement",
    "adapter": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "confidence": 0.85,
    "method": "heuristic",
    "candidates": [
      {"task_type": "implement", "score": 0.85},
      {"task_type": "scaffold", "score": 0.3}
    ],
    "reasoning": "Matched triggers: implement, create"
  }
}
```

---

## Best Practices

### 1. Use Task Types in Pipelines

```yaml
stages:
  - name: research
    task_type: research    # Routes to research-optimized model
    prompt: "Research {{ .Input }}"

  - name: implement
    task_type: implement   # Routes to code-optimized model
    prompt: "Implement based on {{ .Artifacts.research.Text }}"
```

### 2. Set Appropriate Confidence Threshold

- **Higher threshold (0.8+)**: More LLM classifier usage, more accurate routing
- **Lower threshold (0.5-0.7)**: Faster routing, may misclassify edge cases

### 3. Customize for Your Workflow

If you frequently work on specific tasks, customize routing:

```yaml
task_types:
  # Custom task type for your domain
  data_pipeline:
    adapter: anthropic
    model: claude-sonnet-4-20250514
    triggers:
      - "data pipeline"
      - "etl"
      - "transform"
      - "ingest"
```

### 4. Use Aliases for Quick Access

Define aliases for commonly used configurations:

```yaml
aliases:
  review: claude-opus-4-20250514    # Always use best model for reviews
  draft: deepseek-chat              # Use cheap model for drafts
```
