# Sourced, hidden, by demo.tape: a throwaway home with fake jobs (python scripts that only sleep)
# and a sparse 30 GB "model" that takes no disk space.
demo=${TMPDIR:-/tmp}; demo=${demo%/}/mlx-garde-demo
rm -rf "$demo"; mkdir -p "$demo/lab" "$demo/models/Qwen3.8-27B-4bit"; demo=$(cd "$demo" && pwd -P)
export PATH="$PWD/bin:$PATH" HOME="$demo" MLX_GARDE_CONFIG="$demo/config.toml"
python3 -c "open('$demo/models/Qwen3.8-27B-4bit/model.safetensors','wb').truncate(30 << 30)"
echo '{}' > "$demo/models/Qwen3.8-27B-4bit/config.json"
cat > "$demo/lab/embed.py" <<'PY'
import time
print("embedding notes/ with bge-m3", flush=True)
for n in (312, 744, 1204):
    time.sleep(0.6); print(f"  {n} chunks", flush=True)
print("done", flush=True)
PY
for f in transcribe eval finetune bench; do printf 'import time\ntime.sleep(600)\n' > "$demo/lab/$f.py"; done
setopt interactivecomments
PROMPT='%F{8}~/lab%f %F{12}❯%f '
cd "$demo/lab"
