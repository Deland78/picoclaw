# Config Directory

This directory contains the example configuration template.

## Live Config Location

When picoclaw runs, it reads the config from:

```
~/.picoclaw/config.json
```

On Windows: `C:\Users\<username>\.picoclaw\config.json`
e.g. C:\Users\david\.picoclaw\config.json

## Getting Started

Copy the example to create your live config:

```bash
cp config/config.example.json ~/.picoclaw/config.json
```

Then edit `~/.picoclaw/config.json` with your API keys and preferences.

## Files

- `config.example.json` - Template with placeholder values. Safe to commit.
- `config.json` - Gitignored. Do not create here; use `~/.picoclaw/config.json` instead.
