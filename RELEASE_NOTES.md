First production-ready release of Tokligence Gateway - an open-source,
OpenAI-compatible LLM gateway with multi-provider support.

## v0.2.0

See `RELEASE_NOTES_v0.2.0.md` for full details. Highlights:
- Streaming tool bridge (Anthropic → OpenAI)
- Model alias consistency (exact + wildcard)
- Independent Anthropic passthrough toggle
- Tolerant parsing, diagnostics, tests & docs

## 🎯 Key Features

- **Multi-Provider Support**: OpenAI, Anthropic (Claude), and loopback adapters
- **Intelligent Routing**: Pattern-based model routing with wildcards
- **Resilience**: Automatic fallback and retry with exponential backoff
- **Streaming**: Server-Sent Events (SSE) for real-time completions
- **Embeddings API**: Full support for text embeddings
- **Marketplace Integration**: Optional communication with Tokligence Marketplace
- **Production Ready**: 89+ comprehensive tests, all passing

## 📦 Installation

Download the binary for your platform and run:
```bash
./gateway init
./gateway


⚠️ Pre-release Note

This is a pre-1.0 release. While feature-complete and thoroughly tested,
the API may still evolve based on user feedback.
