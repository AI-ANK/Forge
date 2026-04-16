# Forge

**A local-first coding agent you can replay, fork, and share.**

Forge runs on your laptop via [Ollama](https://ollama.com) — no API key, no subscription, no cloud round-trips by default. Every run is recorded to a single SQLite file (`.forge`). Replay a run step by step, fork it mid-way with a new prompt without re-paying, or share the session file with a teammate.

---

## Why another coding agent?

The market is crowded, but two lanes are genuinely unclaimed:

- **No API key required.** Every existing agent assumes Claude or GPT. Forge defaults to `qwen2.5-coder:7b` on Ollama — 4.7 GB, runs on an 8 GB Mac. Cloud escalation is one flag away (coming soon) but opt-in.
- **Fork-mid-run without re-paying.** When an agent goes off the rails at step 14, you stop it, restart from scratch, and pay again. Forge records every LLM response and tool result. `forge fork <id> --at-step 13 "try a different approach"` replays steps 1–13 instantly from the log (zero API calls, zero waiting) and goes live from step 14.

## Install

```bash
# 1. Install Ollama and pull a model (one-time)
brew install ollama
ollama pull qwen2.5-coder:7b

# 2. Install Forge
go install github.com/AI-ANK/Forge/cmd/forge@latest
```

A prebuilt Homebrew tap, Linux packages, and Windows binaries are on the roadmap.

## 30-second demo

```bash
$ forge run "write hello.go that prints hi and a test for it"
forge: session d431d… (seed …, model qwen2.5-coder:7b)
forge: goal: write hello.go that prints hi and a test for it

[turn 1] list existing files
  → glob {"pattern":"**/*.go"}
  ← (no matches)
[turn 2] create main file
  → write_file {"path":"hello.go","content":"package main…"}
  ← wrote 42 bytes to hello.go
[turn 3] create test
  → write_file {"path":"hello_test.go","content":"…"}
  ← wrote 88 bytes to hello_test.go
[turn 4] run tests
  → shell {"cmd":"go test ./..."}
  ← exit=0  PASS

forge: done — Added hello.go and a passing test.
forge: session saved at .forge/sessions.forge (id d431d…)
forge: replay with `forge replay d431d…`
```

Now replay it, even with Ollama offline — recorded bytes are served straight from the log:

```bash
$ forge replay d431d…
forge replay: session d431d…  seed=…  model=qwen2.5-coder:7b
[turn 1] list existing files
  → glob {"pattern":"**/*.go"}
  ← (no matches)
…
forge replay: done — Added hello.go and a passing test.
```

## Commands

| Command | What it does |
|---|---|
| `forge run "<goal>"` | Run the agent on a goal. Records to `.forge/sessions.forge`. |
| `forge replay <id>` | Walk a recorded session step by step. No LLM or tools re-run. |
| `forge sessions` | List recorded sessions in the local store. |
| `forge fork <id> --at-step N "<prompt>"` | (roadmap) Replay to step N, then go live from there. |
| `forge bisect <id>` | (roadmap) Binary-search which step broke a check. |
| `forge share <id>` | (roadmap) Emit a self-contained `.forge` file for others to replay. |

## How determinism works

Every non-deterministic source routes through the session recorder:

- **LLM**: `temperature=0`, fixed `seed` per step (derived from the session seed). Response bytes are recorded verbatim. Replay returns those bytes without re-calling Ollama.
- **Shell**: stdout + stderr + exit code captured byte-for-byte. Replay returns the recorded output rather than re-executing.
- **File reads**: content recorded at read time. Replay reads from the log.
- **Clock / UUID / rand**: all seeded from the session seed.

This means a `.forge` file is a complete, hermetic record of a run. You can replay it on any machine, with Ollama offline, on a plane, years later.

## Status

Milestone 0 (run + replay) is working. The full fork/bisect/share UX and cloud-model escalation are the next milestones. Track progress in the planning doc.

## Stack

Single-binary Go, pure-Go SQLite (`modernc.org/sqlite`, no CGO), `spf13/cobra` CLI. Cross-compiles for macOS, Linux, and Windows in one command.

## Inspiration

Forge exists in a rich ecosystem: [Claude Code](https://www.anthropic.com/claude-code), [Claw Code](https://claw-code.codes/), [OpenCode](https://opencode.ai/), [Aider](https://aider.chat), [Hermes Agent](https://hermes-agent.nousresearch.com/), and the broader [Claw family](https://github.com/openclaw/openclaw) (OpenClaw, NanoClaw, PicoClaw, PiClaw, MetaClaw). Forge's niche is the intersection they leave open: **local-first + time-travel**.

## License

Apache 2.0.
