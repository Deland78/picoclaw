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
| 8 | PicoClaw config changes are speculative | Step 4 cannot be precise without the actual config file | Flagged clearly; added config patterns for all common PicoClaw layouts |
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

The Qwen3.5 repetition bug (models echoing output) was fixed in Ollama v0.17.1+. You must update **and** re-pull models — an Ollama update alone does not fix already-downloaded model files.

**Download:** https://ollama.com/download/windows

Run the Windows installer (`.exe`). It updates in-place, preserving your existing models. Ollama on Windows runs as a **system tray app**, not a manual server — it starts automatically.

After install, verify version:

```powershell
ollama --version
# Must show 0.17.1 or higher
```

### Disable Cloud Routing (Privacy / Data Sovereignty)

By default, Ollama may route certain requests to Ollama Cloud. For PicoClaw's local-only design, disable this:

```powershell
# Set permanently in Windows environment variables
[System.Environment]::SetEnvironmentVariable("OLLAMA_NO_CLOUD", "1", "Machine")

# Verify (requires new PowerShell window after setting)
$env:OLLAMA_NO_CLOUD
```

Alternatively, set it in the Ollama tray app: right-click tray icon → Settings → disable cloud models.

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

Both endpoints should return JSON with the model's response. The OpenAI-compatible endpoint at `/v1` is what most Python frameworks (LangChain, CrewAI, etc.) expect.

---

## Step 5: Thinking Mode — Critical Performance Decision

**This is the most important Qwen3.5-specific consideration for PicoClaw.**

Qwen3.5 has a built-in reasoning chain ("thinking mode") that is **ON by default**. When enabled, the model emits `<think>...</think>` blocks before answering. This improves quality but adds latency — problematic for edge/agent use cases.

### Control Thinking Mode

| Method | When to Use |
|---|---|
| Append `/think` to prompt | Complex reasoning tasks: analysis, code generation |
| Append `/no_think` to prompt | Fast-pass tasks: classification, simple lookups |
| Set in Modelfile | Force a mode at the model level for PicoClaw |

### Recommended: Create a PicoClaw-Optimized Modelfile

Create a file called `Modelfile-picoclaw` in your working directory:

```
FROM qwen3.5:4b

SYSTEM """
You are a focused, precise assistant operating in an edge computing environment.
Respond concisely. Use structured output when appropriate.
/no_think
"""

PARAMETER num_ctx 32768
PARAMETER temperature 0.3
PARAMETER repeat_penalty 1.1
```

Then build and register the custom model:

```powershell
ollama create picoclaw-qwen35 -f Modelfile-picoclaw

# Verify it appears
ollama list

# Test it
ollama run picoclaw-qwen35 "What is 2+2?"
```

Now PicoClaw can call `picoclaw-qwen35` instead of `qwen3.5:4b` — giving you a pre-configured, fast-response version with no thinking overhead by default. Individual skills that need reasoning can still override with `/think` in the prompt.

> **Context window note:** The default Ollama context is often capped at 2048 tokens. The `num_ctx 32768` in the Modelfile sets it to 32K. Qwen3.5 supports up to 262K — but larger context = more RAM usage. Start at 32K and increase if your skills need it.

---

## Step 6: Update PicoClaw Configuration

> **⚠️ Important:** The exact file locations depend on your PicoClaw project structure. The patterns below cover the most common layouts. Share your config file for precise line-by-line changes.

### 6a. YAML-based config (most common PicoClaw pattern)

Look for a file named `config.yaml`, `settings.yaml`, or `.picoclaw/config.yaml`:

```yaml
# Before
model: "llama3.2"           # or whatever was previously set
provider: "ollama"
base_url: "http://localhost:11434"

# After
model: "picoclaw-qwen35"    # your custom Modelfile model
provider: "ollama"
base_url: "http://localhost:11434"
```

### 6b. Python-based config

```python
# Before
LLM_MODEL = "llama3.2"
OLLAMA_BASE_URL = "http://localhost:11434"

# After
LLM_MODEL = "picoclaw-qwen35"
OLLAMA_BASE_URL = "http://localhost:11434"
# OpenAI-compatible alternative:
OLLAMA_BASE_URL_V1 = "http://localhost:11434/v1"
```

### 6c. Per-skill model override (if your YAML skills support model frontmatter)

If individual skills specify a model, update each one:

```yaml
# Before
model: "llama3.2"

# After (use the custom model for most skills)
model: "picoclaw-qwen35"

# Or for skills needing heavy reasoning, use the base model with think mode
model: "qwen3.5:9b"   # if you pulled it
# Add to the skill prompt:
# "...your task... /think"
```

---

## Step 7: Test Vision Capability

Qwen3.5's native multimodal support is new functionality for PicoClaw. Test it before building any vision-dependent skills.

### Test via API with a local image:

```python
import base64, requests

# Load any image from your machine
with open("test_image.jpg", "rb") as f:
    image_b64 = base64.b64encode(f.read()).decode()

payload = {
    "model": "qwen3.5:4b",
    "messages": [{
        "role": "user",
        "content": [
            {"type": "text", "text": "Describe this image in one sentence. /no_think"},
            {"type": "image_url", "image_url": {"url": f"data:image/jpeg;base64,{image_b64}"}}
        ]
    }],
    "stream": False
}

response = requests.post("http://localhost:11434/api/chat", json=payload)
print(response.json()["message"]["content"])
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

*Plan version: 1.1 | Gap review completed March 3, 2026*
