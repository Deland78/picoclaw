# Review: PicoClaw Qwen3.5 Migration Plan

**Reviewed:** March 4, 2026 | **Scope:** Factual accuracy, codebase alignment, and completeness

---

## Critical Issues

### 1. `/no_think` and `/think` tags do NOT work in Qwen 3.5

> [!CAUTION]
> **Step 5 is built on an incorrect foundation.** Qwen 3.5 has **dropped official support** for the `/think` and `/no_think` soft-switch tags that worked in Qwen 3. The plan recommends putting `/no_think` in the system prompt of the Modelfile and using `/think` in skill prompts -- neither will work reliably.

**Fix:** Replace the `/no_think` approach with one of:
- Setting `"enable_thinking": false` in the API request body (for the OpenAI-compatible endpoint)
- Using Ollama Modelfile parameters: `PARAMETER enable_thinking false`
- Or stripping `<think>` tags in PicoClaw's response handler as a fallback

---

### 2. Step 6 config examples are completely wrong for this codebase

> [!CAUTION]
> The plan shows YAML config, Python config, and per-skill YAML frontmatter -- **none of these exist in PicoClaw.** The actual configuration is:

| What the plan says | What PicoClaw actually uses |
|---|---|
| `config.yaml` / `settings.yaml` | `~/.picoclaw/config.json` (JSON) |
| `model: "picoclaw-qwen35"` | `model_list` array with `ModelConfig` objects |
| `provider: "ollama"` | Protocol prefix: `"model": "ollama/qwen3.5:4b"` |
| Python `LLM_MODEL = ...` | Go binary, no Python config |

**Fix:** The correct config change is to add an entry to the `model_list` array in `~/.picoclaw/config.json`:

```json
{
  "model_name": "picoclaw-qwen35",
  "model": "ollama/picoclaw-qwen35",
  "api_base": "http://localhost:11434/v1",
  "api_key": "ollama"
}
```

And set the default model:
```json
"agents": {
  "defaults": {
    "model": "picoclaw-qwen35"
  }
}
```

The live config is at `C:\Users\david\.picoclaw\config.json` (not `config/config.json` in the repo, which is a reference copy).

---

### 3. Repetition bug fix is oversimplified

> [!WARNING]
> Step 2 says "update Ollama and re-pull models" is the full fix. Research shows the repetition bug also requires **parameter tuning**, specifically `presence_penalty`. Community reports confirm re-pulling alone does not always resolve the issue.

**Fix:** Add to the Modelfile:
```
PARAMETER presence_penalty 1.1
```
And add a troubleshooting note: if repetition persists after re-pulling, adjust `presence_penalty` (1.0-1.5) and `repeat_penalty` (1.1-1.3).

---

## Important Improvements

### 4. RAM/VRAM estimates for `num_ctx 32768` are missing

The plan recommends `num_ctx 32768` (Step 5) but gives **no memory estimate** for this setting. Research shows:

| Model | KV cache at 32K context (Q4) | Total RAM needed |
|---|---|---|
| `qwen3.5:4b` | ~3-4 GB additional | ~6-7 GB total |
| `qwen3.5:9b` | ~5-6 GB additional | ~11-12 GB total |

The 4B model with 32K context and an unquantized KV cache was reported using **13 GB VRAM**. The plan should include a warning and recommend starting with `num_ctx 8192` for laptops with 16 GB RAM or less.

### 5. Vision test code uses wrong API format for Ollama

Step 7's Python test uses the **OpenAI-compatible multipart format** (`image_url` with base64 data URI), but sends it to Ollama's **native API** endpoint (`/api/chat`). Ollama's native API uses a different image format:

```json
{
  "model": "qwen3.5:4b",
  "messages": [{
    "role": "user",
    "content": "Describe this image",
    "images": ["base64-encoded-image-here"]
  }]
}
```

**Fix:** Either use the native format at `/api/chat`, or switch the endpoint to `/v1/chat/completions` with the OpenAI-compatible format.

### 6. Missing: PicoClaw's `model_list` supports fallback chains

PicoClaw's `AgentModelConfig` supports `"primary"` and `"fallbacks"` fields. The plan should recommend using this for graceful degradation:

```json
"model": {
  "primary": "picoclaw-qwen35",
  "fallbacks": ["llama3"]
}
```

This provides automatic rollback without manual config changes if Qwen3.5 fails.

### 7. Missing: `context_window` field in `ModelConfig`

The `ModelConfig` struct has a `context_window` field. When adding the Qwen3.5 model to `model_list`, this should be set:

```json
"context_window": 32768
```

This lets PicoClaw properly manage prompt truncation rather than relying only on the Ollama Modelfile `num_ctx`.

---

## Minor Improvements

### 8. Ollama version number `0.17.1` is unverifiable
The plan hard-codes version `0.17.1` as the fix version. This should be changed to "check the Ollama releases page for the latest version with Qwen3.5 repetition fix" since version numbers may differ.

### 9. `OLLAMA_NO_CLOUD` tray app setting not confirmed
The plan says: *"right-click tray icon -> Settings -> disable cloud models."* This UI option may not exist in all Ollama versions. The environment variable approach (`OLLAMA_NO_CLOUD=1`) is the reliable method; the tray UI instruction should be marked as "if available."

### 10. Add `OLLAMA_FLASH_ATTENTION=1` recommendation
Community reports show `OLLAMA_FLASH_ATTENTION=1` significantly improves inference speed and can help with repetition on supported hardware (NVIDIA GPUs with compute capability >= 8.0). Worth mentioning as an optional optimization.

### 11. Modelfile should include stop sequences
Qwen3.5 can loop if stop sequences are misconfigured. The Modelfile should explicitly include:
```
PARAMETER stop "<|im_end|>"
PARAMETER stop "
