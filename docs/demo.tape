# Demo GIF for the README, from the repo root: go build -o bin/mlx-garde ./cmd/mlx-garde && vhs docs/demo.tape
# The jobs are python scripts that only sleep and the "model" is a sparse 30 GB file: nothing heavy runs.
Output docs/demo.gif

Set Shell zsh
Set FontSize 18
Set Width 1300
Set Height 760
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type "source docs/demo-setup.sh && clear" Enter
Sleep 1s
Show

Type "# a quick job: run, then swap checked" Enter
Sleep 600ms
Type "mlx-garde -- python3 embed.py" Enter
Sleep 3.5s

Type "# load a 30 GB model with 28 GB free? nope" Enter
Sleep 600ms
Type "mlx-garde --model ~/models/Qwen3.8-27B-4bit -- python3 bench.py" Enter
Sleep 4s

Hide
Type "mlx-garde --need 20 -- python3 transcribe.py meeting.wav >/dev/null 2>&1 &!" Enter
Sleep 800ms
Type "mlx-garde -- python3 eval.py --sample 50 >/dev/null 2>&1 &!" Enter
Sleep 800ms
Type "clear" Enter
Sleep 300ms
Show

Type "# something heavy is running: no piling on" Enter
Sleep 600ms
Type "mlx-garde --no-wait -- python3 finetune.py" Enter
Sleep 3s

Type "mlx-garde status --watch" Enter
Sleep 7s
Type "q"
Sleep 1.2s

Hide
Type "pkill -f 'python3 (transcribe|eval).py'" Enter
Sleep 500ms
