# PicoClaw → Qwen3.5 Migration Plan
**Platform:** Windows 11 Laptop | **Date:** March 2026  
**Models:** Qwen3.5 Small Series (0.8B / 2B / 4B / 9B) via Ollama

---

## Gap Analysis: Issues Found in the Original Plan

Before the step-by-step plan, here is a summary of problems identified and corrected:

| # | Gap / Problem | Impact | Fix Applied |
|---|---|---|---|
| 1 | No hardware check before model selection | Risk of pulling a model that OOMs the laptop | Added hardware check step (Step 1) |
| 2 | Ollama on Windows runs as a tray app, not `ollama serve` | `ollama serve` command is for Linux/Mac; on Windows it's automatic | Removed incorrect instruction, added Windows-specific note |
| 3 | Source contradiction: "no Qwen3.5 GGUF works in Ollama" | This applies only to the large MoE models (35B+), not the small series | Clarified: 0.8B–9B work fine in Ollama; large models need llama.cpp |
| 4 | Thinking mode is ON by default | Adds significant latency on CPU-only laptops; PicoClaw edge use cases need fast response | Added explicit thinking mode management section |
| 5 | Repetition bug requires model re-pull, not just Ollama update | Old pulls will still exhibit the bug even after updating Ollama | Added explicit `ollama pull` after update |
| 6 | No rollback / comparison step | If Qwen3.5 underperforms, there is no path back without re-downloading the old model | Added side-by-side validation and rollback note |
| 7 | Windows Defender / Firewall can block localhost:11434 | Silent API failures that look like PicoClaw bugs | Added firewall check step |
| 8 | PicoClaw config changes are speculative | Step 4 cannot be precise without the actual config file | Fixed: Step 6 now uses exact JSON `model_list` format from the codebase |
| 9 | No quantization guidance | Default pull may not be optimal for available VRAM/RAM | Added quantization options table |
| 10 | Vision capability not confirmed working | Multimodal is the headline feature; worth verifying it actually works in your Ollama version | Added dedicated vision test with a real example |
| 11 | `OLLAMA_NO_CLOUD=1` not mentioned | Privacy/data-sovereignty is a core PicoClaw value; cloud routing should be explicitly disabled | Added environment variable setup |
| 12 | No context window configuration guidance | 262K context is a major Qwen3.5 advantage; default Ollama may cap it lower | Added `num_ctx` parameter note |

---

## Prerequisites

- Ollama already installed on Windows 11 ✓
- PicoClaw running locally ✓
- Windows PowerShell (run as Administrator for some steps)

---

## Step 1: Check Your Hardware First

**Do this before pulling any model.** Model choice depends on your available RAM.

Open PowerShell and run:

```powershell
# Check total and available RAM
Get-CimInstance Win32_ComputerSystem | Select-Object TotalPhysicalMemory
[math]::Round((Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory / 1MB, 1)

# Check for discrete GPU (NVIDIA)
nvidia-smi
# If that errors, you're CPU-only — important for speed expectations
```

### Model Selection Table

| Your Available RAM | Recommended Model | Download Size | Speed on CPU-only |
|---|---|---|---|
| 4–5 GB free | `qwen3.5:0.8b` | ~600 MB | Fast (~15–25 tok/s) |
| 6–7 GB free | `qwen3.5:2b` | ~1.5 GB | Good (~10–18 tok/s) |
| 8–11 GB free | `qwen3.5:4b` | ~3 GB | Moderate (~6–12 tok/s) |
| 12+ GB free | `qwen3.5:9b` | ~6.6 GB | Slow on CPU (~2–5 tok/s) |

> **Note:** If you have an NVIDIA GPU with 8+ GB VRAM, run `qwen3.5:9b` without hesitation — GPU inference is 10–20x faster than CPU-only. The 4B fits in 6 GB VRAM.

> **Quantization options:** The default `ollama pull qwen3.5:4b` pulls a Q4 quantized version (good balance). You can also explicitly pull higher quality: `qwen3.5:4b-instruct-q8_0` for better quality at 2x the size, or `qwen3.5:4b-instruct-fp16` for full precision (requires 2x more RAM). Start with the default.

---

## Step 2: Update Ollama

The Qwen3.5 repetition bug (models echoing output) was fixed in a recent Ollama release. Check the [Ollama releases page](https://github.com/ollama/ollama/releases) for the latest version with the Qwen3.5 repetition fix. You must update **and** re-pull models — an Ollama update alone does not fix already-downloaded model files.

**Download:** https://ollama.com/download/windows

Run the Windows installer (`.exe`). It updates in-place, preserving your existing models. Ollama on Windows runs as a **system tray app**, not a manual server — it starts automatically.

After install, verify version:

```powershell
ollama --version
# Must be the latest version with the Qwen3.5 repetition fix
```

### Disable Cloud Routing (Privacy / Data Sovereignty)

By default, Ollama may route certain requests to Ollama Cloud. For PicoClaw's local-only design, disable this:

```powershell
# Set permanently in Windows environment variables
[System.Environment]::SetEnvironmentVariable("OLLAMA_NO_CLOUD", "1", "Machine")

# Verify (requires new PowerShell window after setting)
$env:OLLAMA_NO_CLOUD
```

Alternatively, if available in your Ollama version: right-click the tray icon → Settings → disable cloud models. The environment variable approach above is the reliable method across all versions.

---

### Optional: Enable Flash Attention (NVIDIA GPUs)

If you have an NVIDIA GPU with compute capability >= 8.0 (RTX 30xx+), Flash Attention can significantly improve inference speed and help with repetition:

```powershell
[System.Environment]::SetEnvironmentVariable("OLLAMA_FLASH_ATTENTION", "1", "Machine")
```

Restart Ollama after setting this.

---

## Step 3: Pull Qwen3.5 Models

**Ollama model library:** https://ollama.com/library/qwen3.5

```powershell
# Pull your primary model (replace 4b with your choice from Step 1)
ollama pull qwen3.5:4b

# IMPORTANT: Even if you had a Qwen3.5 model before, re-pull it
# to get the repetition bug fix
ollama pull qwen3.5:9b   # optional, only if RAM supports it

# Verify downloads
ollama list
```

Models are stored at: `C:\Users\<YourUsername>\.ollama\models\`

> **HuggingFace alternative:** If you want base weights (for fine-tuning, not Ollama inference):
> - 4B: https://huggingface.co/Qwen/Qwen3.5-4B-Instruct
> - 9B: https://huggingface.co/Qwen/Qwen3.5-9B-Instruct
> - GGUF variants (llama.cpp): https://huggingface.co/unsloth/Qwen3.5-4B-GGUF

---

## Step 4: Verify Ollama API Is Accessible

PicoClaw communicates with Ollama via REST API at `http://localhost:11434`.

### 4a. Check Windows Firewall

Ollama on Windows may be blocked by Windows Defender Firewall for localhost traffic in some configurations. Test first:

```powershell
# Basic connectivity test
Invoke-WebRequest -Uri "http://localhost:11434/api/tags" -Method GET
```

If this errors (connection refused), check:

```powershell
# Check if Ollama is listening
netstat -ano | findstr "11434"

# If not listening, restart Ollama from tray icon
# If listening but blocked, add firewall exception:
New-NetFirewallRule -DisplayName "Ollama Local" -Direction Inbound -Protocol TCP -LocalPort 11434 -Action Allow
```

### 4b. Test Chat API with Qwen3.5

```powershell
# Test via Ollama native API
curl http://localhost:11434/api/chat -d '{\"model\": \"qwen3.5:4b\", \"messages\": [{\"role\": \"user\", \"content\": \"Respond with exactly: API working.\"}], \"stream\": false}'

# Test via OpenAI-compatible API (used by many frameworks)
curl http://localhost:11434/v1/chat/completions -H "Content-Type: application/json" -d '{\"model\": \"qwen3.5:4b\", \"messages\": [{\"role\": \"user\", \"content\": \"API test\"}]}'
```

Both endpoints should return JSON with the model's response. **PicoClaw uses the OpenAI-compatible endpoint** (`/v1/chat/completions`) — the native `/api/chat` test above is just to verify Ollama is running.

---

## Step 5: Thinking Mode — Critical Performance Decision

**This is the most important Qwen3.5-specific consideration for PicoClaw.**

Qwen3.5 has a built-in reasoning chain ("thinking mode") that is **ON by default**. When enabled, the model emits `<think>...</think>` blocks before answering. This improves quality but adds latency — problematic for edge/agent use cases.

### Control Thinking Mode

> **⚠️ Important:** Qwen 3.5 has **dropped official support** for the `/think` and `/no_think` soft-switch tags that worked in Qwen 3. Do not use these in prompts or system messages — they will not work reliably.

| Method | When to Use |
|---|---|
| `PARAMETER enable_thinking false` in Modelfile | Disable thinking at the model level for PicoClaw **(recommended — only method that works without code changes)** |
| Strip `<think>` tags in response handler | Fallback if thinking output leaks through **(requires adding code — see note below)** |

> **⚠️ PicoClaw does not currently pass `enable_thinking` in API requests.** The `openai_compat` provider only forwards `max_tokens` and `temperature`. Using `"enable_thinking": false` in the request body would require modifying `pkg/providers/openai_compat/provider.go`. The Modelfile parameter is the only zero-code-change option.
>
> **⚠️ PicoClaw does not currently strip `<think>` tags from responses.** If thinking output leaks through despite the Modelfile setting, `<think>...</think>` blocks will appear in user-facing output. To add stripping as a safety net, a filter would need to be added in `pkg/providers/openai_compat/provider.go` (in `parseResponse`) or in `pkg/agent/loop.go` (after the LLM call).

### Recommended: Create a PicoClaw-Optimized Modelfile

Create a file called `Modelfile-picoclaw` in your working directory:

```
FROM qwen3.5:4b

SYSTEM """
You are a focused, precise assistant operating in an edge computing environment.
Respond concisely. Use structured output when appropriate.
"""

PARAMETER enable_thinking false
PARAMETER num_ctx 32768
PARAMETER temperature 0.3
PARAMETER repeat_penalty 1.1
PARAMETER presence_penalty 1.1
PARAMETER stop "<|im_end|>"
PARAMETER stop "<|endoftext|>"
```

> **Repetition troubleshooting:** If repetition persists after re-pulling the model, adjust `presence_penalty` (1.0–1.5) and `repeat_penalty` (1.1–1.3) until output stabilizes.

Then build and register the custom model:

```powershell
ollama create picoclaw-qwen35 -f Modelfile-picoclaw

# Verify it appears
ollama list

# Test it
ollama run picoclaw-qwen35 "What is 2+2?"
```

Now PicoClaw can call `picoclaw-qwen35` instead of `qwen3.5:4b` — giving you a pre-configured, fast-response version with no thinking overhead by default. Individual skills that need reasoning can override with `"enable_thinking": true` in the API request body.

> **Context window note:** The default Ollama context is often capped at 2048 tokens. The `num_ctx 32768` in the Modelfile sets it to 32K. Qwen3.5 supports up to 262K — but larger context = more RAM usage.

### RAM Impact of `num_ctx 32768`

| Model | KV cache at 32K context (Q4) | Total RAM needed |
|---|---|---|
| `qwen3.5:4b` | ~3–4 GB additional | ~6–7 GB total |
| `qwen3.5:9b` | ~5–6 GB additional | ~11–12 GB total |

> **Warning:** The 4B model with 32K context and an unquantized KV cache has been reported using **13 GB VRAM**. If you have 16 GB RAM or less, start with `num_ctx 8192` and increase only if needed.

---

## Step 6: Update PicoClaw Configuration

PicoClaw uses a JSON config file at `~/.picoclaw/config.json` (on Windows: `C:\Users\<YourUsername>\.picoclaw\config.json`). The Go binary reads a `model_list` array of `ModelConfig` objects, with model names using the `protocol/modelID` prefix format.

### 6a. Add Qwen3.5 to model_list

Add an entry to the `model_list` array in `~/.picoclaw/config.json`:

```json
{
  "model_name": "picoclaw-qwen35",
  "model": "ollama/picoclaw-qwen35",
  "api_base": "http://localhost:11434/v1",
  "api_key": "",
  "context_window": 32768
}
```

> **Note on `api_key`:** Ollama does not require authentication. The factory only requires that `api_key` and `api_base` aren't both empty — since `api_base` is set, `api_key` should be `""`. Do not set it to `"ollama"` as that would send a pointless `Authorization` header.

> **Note:** The `context_window` field lets PicoClaw properly manage prompt truncation rather than relying only on the Ollama Modelfile `num_ctx`. Set this to match your Modelfile setting.

### 6b. Set as default model

In the same `config.json`, set the default agent model. Choose **one** of these two formats — they are alternatives for the same `model` field:

**Simple (no fallback):**
```json
"agents": {
  "defaults": {
    "model": "picoclaw-qwen35"
  }
}
```

**With fallback chain (recommended):**
```json
"agents": {
  "defaults": {
    "model": {
      "primary": "picoclaw-qwen35",
      "fallbacks": ["llama3"]
    }
  }
}
```

This provides automatic rollback if Qwen3.5 fails.

> **⚠️ Fallback limitation:** The fallback chain uses the same provider instance for all candidates. All fallback models must be available on the same endpoint (e.g., all on the same Ollama instance). Cross-provider fallback (e.g., Ollama → OpenRouter) is not supported without code changes. Make sure any model listed in `fallbacks` also has an entry in `model_list` with the same `api_base`.

---

## Step 7: Test Vision Capability

Qwen3.5's native multimodal support is new functionality for PicoClaw. Test it before building any vision-dependent skills.

### Test via Ollama native API with a local image:

```python
import base64, requests

# Load any image from your machine
with open("test_image.jpg", "rb") as f:
    image_b64 = base64.b64encode(f.read()).decode()

# Ollama native API format (/api/chat) uses "images" array, not OpenAI's "image_url"
payload = {
    "model": "qwen3.5:4b",
    "messages": [{
        "role": "user",
        "content": "Describe this image in one sentence.",
        "images": [image_b64]
    }],
    "stream": False
}

response = requests.post("http://localhost:11434/api/chat", json=payload)
print(response.json()["message"]["content"])
```

### Alternative: Test via OpenAI-compatible endpoint:

```python
import base64, requests

with open("test_image.jpg", "rb") as f:
    image_b64 = base64.b64encode(f.read()).decode()

# OpenAI-compatible format (/v1/chat/completions) uses "image_url" with data URI
payload = {
    "model": "qwen3.5:4b",
    "messages": [{
        "role": "user",
        "content": [
            {"type": "text", "text": "Describe this image in one sentence."},
            {"type": "image_url", "image_url": {"url": f"data:image/jpeg;base64,{image_b64}"}}
        ]
    }]
}

response = requests.post("http://localhost:11434/v1/chat/completions", json=payload)
print(response.json()["choices"][0]["message"]["content"])
```

> **Note:** If vision fails (model returns "I cannot see images"), your Ollama version may not yet have full Qwen3.5 multimodal support active. Check https://github.com/ollama/ollama/releases for the latest patch notes. The model weights support vision; Ollama's serving layer must also support it.

---

## Step 8: Validate Against Current Behavior (Rollback Safety)

Before removing your old model, run the same test prompts against both models and compare.

```powershell
# Keep the old model available as a fallback
ollama list   # note what was there before

# Run the same prompt on both
ollama run llama3.2 "Summarize this task: [your typical PicoClaw task]"
ollama run picoclaw-qwen35 "Summarize this task: [your typical PicoClaw task]"
```

Evaluate:
- Is output quality equal or better?
- Is latency acceptable for your workflow?
- Do any skills produce malformed output (unexpected `<think>` tags leaking through)?

**If Qwen3.5 underperforms:** do not remove the old model yet. You can continue using it by reverting the model name in config. Only remove old models once you've run PicoClaw through a full real workload.

```powershell
# Only when fully validated:
ollama rm llama3.2   # replace with your actual old model name
```

---

## Step 9: Model Management on Windows

```powershell
# List all models and sizes
ollama list

# Free VRAM/RAM when not using the model (important on laptops)
ollama stop picoclaw-qwen35

# Update a model to latest version
ollama pull qwen3.5:4b

# Remove a model to reclaim disk space
ollama rm qwen3.5:2b

# See running models and their memory usage
ollama ps
```

> **Laptop battery tip:** Ollama on Windows keeps the loaded model hot in RAM between requests. If you're not using PicoClaw for a while, stop the model to reclaim RAM and reduce battery draw.

---

## Quick Reference

| Resource | URL |
|---|---|
| Ollama Windows download | https://ollama.com/download/windows |
| Qwen3.5 Ollama model page | https://ollama.com/library/qwen3.5 |
| Qwen3.5-4B HuggingFace | https://huggingface.co/Qwen/Qwen3.5-4B-Instruct |
| Qwen3.5-9B HuggingFace | https://huggingface.co/Qwen/Qwen3.5-9B-Instruct |
| GGUF variants (llama.cpp / fine-tuning) | https://huggingface.co/unsloth/Qwen3.5-4B-GGUF |
| Ollama API reference | https://github.com/ollama/ollama/blob/main/docs/api.md |
| Ollama release notes (bug tracking) | https://github.com/ollama/ollama/releases |
| Qwen3.5 official model card | https://huggingface.co/Qwen/Qwen3.5-9B-Instruct#introduction |

---

## What This Does NOT Cover

- **Fine-tuning Qwen3.5** on PicoClaw-specific data (requires a separate GPU workflow using Unsloth or Axolotl)
- **PicoClaw skill rewrites** to take advantage of Qwen3.5's 262K context window
- **Vision skill development** — the API is confirmed above, but building vision-aware PicoClaw skills is a separate design task
- **Multi-model routing** — using 0.8B for fast tasks and 9B for complex ones within a single PicoClaw session

---

*Plan version: 1.3 | Updated March 4, 2026 — codebase audit: fixed api_key, clarified enable_thinking/think-tag limitations, fallback chain constraints, endpoint usage*
