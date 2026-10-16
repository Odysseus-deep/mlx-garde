#!/bin/zsh
# Claude Code PreToolUse hook (matcher: Bash): refuse a heavy local MLX job that does not go through mlx-garde.
# Declare it in ~/.claude/settings.json:
#   "hooks": { "PreToolUse": [ { "matcher": "Bash", "hooks": [ { "type": "command", "command": "~/.claude/hooks/mlx-garde.sh" } ] } ] }
c=$(jq -r '.tool_input.command // empty')
[[ -z $c || $c == *mlx-garde* ]] && exit 0
print -r -- "$c" | grep -qE '(^|[^a-z])mlx_(lm|vlm)([^a-z]|$)|mlx-lm' || exit 0
jq -n '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny",
  permissionDecisionReason: "Heavy local MLX job: prefix it with `mlx-garde --model <model dir> --` (one job at a time, refused if RAM is short, swap report). Current state: `mlx-garde status`."}}'
