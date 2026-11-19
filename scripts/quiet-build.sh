#!/bin/sh
# quiet-build.sh LABEL LOGFILE -- COMMAND [ARGS...]
# Runs a noisy build with its output in LOGFILE, and shows one line instead:
# "▸ Building LABEL…" (a spinner on a terminal), then "✔ Built LABEL in 42s".
# If it fails, shows the last lines of the log and where the whole log is, and
# exits with the command's own status.
label=$1; log=$2; shift 3
. "$(dirname "$0")/pretty.sh"

# One build at a time: wails writes and removes the same files (the Windows
# resource .syso, build/bin, the log) in this folder, so a second build at once
# breaks the first. A second one waits; a lock left by a build that died is
# taken over.
lock="$(dirname "$0")/../.build.lock"
waited=0
while ! mkdir "$lock" 2>/dev/null; do
  holder=$(cat "$lock/pid" 2>/dev/null)
  if [ -n "$holder" ] && ! kill -0 "$holder" 2>/dev/null; then
    rm -rf "$lock"
    continue
  fi
  if [ "$waited" -eq 0 ]; then say_step "Another build is running here (pid ${holder:-?}); waiting for it…"; fi
  waited=1
  sleep 2
done
echo $$ >"$lock/pid"
trap 'rm -rf "$lock"' EXIT INT TERM

start=$(date +%s)
"$@" >"$log" 2>&1 &
pid=$!
if [ -t 1 ] && [ -z "$NO_COLOR" ]; then
  i=0
  while kill -0 "$pid" 2>/dev/null; do
    case $((i % 4)) in 0) f='⠋';; 1) f='⠙';; 2) f='⠹';; *) f='⠸';; esac
    printf '\r%s %s%s%s %s… %s%ss%s ' "$B_BUILD" "$C_CYAN" "$f" "$C_RESET" "$label" "$C_DIM" "$(( $(date +%s) - start ))" "$C_RESET"
    i=$((i + 1)); sleep 0.15
  done
  printf '\r\033[K'
else
  say_step "Building ${label}…"
fi
wait "$pid"; rc=$?
secs=$(( $(date +%s) - start ))
if [ "$rc" -eq 0 ]; then
  say_ok "Built $label in ${secs}s, at $(date '+%H:%M:%S')"
else
  say_fail "Build failed: $label (after ${secs}s, at $(date '+%H:%M:%S'))"
  tail -n 30 "$log"
  say_dim "Full build log: $log"
fi
exit "$rc"
