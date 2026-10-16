# mlx-garde

Run heavy local LLM jobs on your Mac one at a time, refuse the ones that won't fit in RAM, and know when a run made it swap.

Apple Silicon makes it tempting to run everything locally: a 27B model in oMLX, a transcription job with Whisper and pyannote, an embedding batch, an eval script your coding agent just wrote. Start two of them at once and macOS quietly swaps tens of gigabytes to the SSD. Nothing crashes, everything crawls, and you find out an hour later.

`mlx-garde` sits in front of those jobs:

1. **One heavy job at a time.** Others wait in a queue. The lock is a `flock(2)`, so a job killed with `kill -9` never leaves the queue stuck.
2. **No load that won't fit.** With `--model <dir>`, it refuses if a local server (oMLX, Ollama, LM Studio) already holds a model, or if available memory is below *model size × 1.3 + 6 GB*. With `--need <GB>`, it checks a declared need instead.
3. **Swap report.** It counts pages actually written to swap during the run (swapouts, not the file-backed pageouts that `vm_stat` also lists) and flags any run that wrote more than 256 MB.

```
$ mlx-garde --model ~/.omlx/models/Qwen3.8-27B-4bit -- python bench.py
mlx-garde: refused, a local server already holds omlx:Qwen3.8-27B-4bit; go through its API or unload it first.

$ mlx-garde --need 20 -- python transcribe.py meeting.wav
...
mlx-garde: swap OK (0 MB written, swap 0 MB → 0 MB)
```

## Live view

```
$ mlx-garde status --watch
╭──────────────────────────────────────────────────────────────────────────────────────╮
│ mlx-garde  23:50:00  q to quit                                                       │
│                                                                                      │
│ memory    ███████████████████████████████████████████░░░░░░░  9.0 GB free of 64      │
│ swap                                            ▁▁▁▁▂▄█▆▂▁▁▁  1200 MB used, 0 MB/s   │
│                                                                                      │
│ job       python transcribe.py --language fr salon.wav  (pid 4242, 7m0s, in ~/code/… │
│ queue 1   python eval.py --sample 20  (pid 4250, 2m0s, in ~/code/histos)             │
│ loaded    omlx:Qwen3.8-27B-4bit                                                      │
│                                                                                      │
│       31.0 GB  python3.12                                                            │
│        9.0 GB  OrbStack Helper                                                       │
╰──────────────────────────────────────────────────────────────────────────────────────╯
```

`mlx-garde status` prints the same once, for scripts.

## Install

```
brew install --cask Odysseus-deep/tap/mlx-garde
```

Or from source (Go and the Xcode command line tools):

```
go install github.com/Odysseus-deep/mlx-garde/cmd/mlx-garde@latest
```

macOS on Apple Silicon only: memory is read from the Mach kernel (`host_statistics64`, `vm.swapusage`).

## Usage

```
mlx-garde [--model <dir> | --need <GB>] [--no-wait] [--] <command…>
mlx-garde status [--watch]
```

| Option | Meaning |
|---|---|
| `--model <dir>` | The command loads this model itself. Refuse if a local server already holds a model, or if memory is short. Symlinks are followed, so Hugging Face cache folders work. |
| `--need <GB>` | The command loads its models on its own (several, or not from one folder). Refuse if available memory < need + margin. |
| *(neither)* | Queue and swap report only: for jobs that go through a server's API. |
| `--no-wait` | Exit with code 75 instead of queueing. |

Exit codes: the command's own code; `75` when refused; `2` for a usage error. Every run is logged to `~/.cache/mlx-garde/journal.log`.

"Available" memory is free + inactive + speculative + purgeable pages: what a new process can take without pushing anything to swap.

## Configuration

Optional, in `~/.config/mlx-garde/config.toml` (or `$MLX_GARDE_CONFIG`). See [`config.example.toml`](config.example.toml). Turn off a server whose default port is taken by something else on your machine.

## Keep your coding agent in line

Agents happily launch `python -m mlx_lm.generate …` while your local server already holds 30 GB. A [Claude Code](https://docs.claude.com/en/docs/claude-code) `PreToolUse` hook can refuse heavy commands that don't go through `mlx-garde`: see [`examples/claude-code-hook.sh`](examples/claude-code-hook.sh).

## License

MIT
