# mlx-garde

**Making rocks do maths is cool. Making them do it from swap is not.**

![mlx-garde demo](docs/demo.gif)

A Mac with 64 GB of unified memory and a fast SSD has one bad habit: it never says no. Load a 27B model in oMLX, start a Whisper transcription, let your coding agent launch "a quick eval", and macOS pages 30 GB out to disk. Nothing crashes, nothing warns you. Everything just crawls, and you notice an hour later.

`mlx-garde` sits in front of those jobs:

- **One heavy job at a time.** The others wait in a queue. The lock is a `flock(2)`, so a job killed with `kill -9` releases it.
- **No model that doesn't fit.** `--model <dir>` takes the model's size on disk, multiplies it by 1.3, adds a 6 GB margin, and compares with what is actually free. It also refuses if oMLX, Ollama or LM Studio already holds a model: two copies of the same weights in memory is rarely what you meant.
- **A swap report for every run.** Pages written to swap are counted before and after (swapouts, not the file-backed pageouts `vm_stat` also lists). Above 256 MB, the run is flagged.

```
$ mlx-garde --model ~/models/Qwen3.8-27B-4bit -- python3 bench.py
mlx-garde: refused, needs about 45.0 GB, 28.5 GB available. Biggest memory users:
    9.6 GB  OrbStack Helper
    2.9 GB  next-server (v16.3.8)
```

## Install

```
brew install --cask Odysseus-deep/tap/mlx-garde
```

Or from source, with Go and the Xcode command line tools:

```
go install github.com/Odysseus-deep/mlx-garde/cmd/mlx-garde@latest
```

Apple Silicon only. Memory is read from the Mach kernel (`host_statistics64`, `vm.swapusage`) rather than by parsing `vm_stat`.

## Usage

Prefix any memory-hungry command:

```
mlx-garde [--model <dir> | --need <GB>] [--no-wait] [--] <command…>
mlx-garde status [--watch]
```

| You run… | Use | mlx-garde checks |
|---|---|---|
| a script that loads one model itself (`mlx_lm`, `transformers`…) | `--model <dir>` | no model already loaded elsewhere, and size × 1.3 + margin fits |
| a job that loads several models (Whisper + pyannote, say) | `--need <GB>` | need + margin fits |
| a job that talks to a running server's API | nothing | queue and swap report only |

- `--no-wait` exits with code 75 instead of queueing.
- Your command's exit code is passed through. `75` means refused, `2` means you typed something wrong.
- Every run is logged in `~/.cache/mlx-garde/journal.log`.
- "Available" means free + inactive + speculative + purgeable pages: what a new process can take without pushing anything to swap.

### Watch it live

`mlx-garde status --watch` shows memory, swap second by second, the running job, the queue, loaded models and the biggest memory users. `q` to quit. `mlx-garde status` prints the same thing once, for scripts.

## Configuration

Optional: `~/.config/mlx-garde/config.toml` (or `$MLX_GARDE_CONFIG`). Margins, the size factor, the swap alarm threshold and which local servers to ask. See [`config.example.toml`](config.example.toml). If something else listens on LM Studio's port 1234, disable that server.

## Coding agents

Agents readily launch `python -m mlx_lm.generate …` while a server already holds 30 GB. A [Claude Code](https://docs.claude.com/en/docs/claude-code) `PreToolUse` hook can refuse heavy commands that don't go through `mlx-garde`: see [`examples/claude-code-hook.sh`](examples/claude-code-hook.sh). The agent is told why and retries with the prefix.

## Why "garde"?

French for *guard*. It started as a zsh script on a Mac mini that swapped 26 GB one afternoon and had to be rebooted.

## The demo

The GIF is recorded with [VHS](https://github.com/charmbracelet/vhs) from [`docs/demo.tape`](docs/demo.tape). The jobs in it are Python scripts that only sleep, and the 30 GB model is a sparse file.

## License

MIT
