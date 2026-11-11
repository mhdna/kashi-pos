# Coloured one-line messages for the release scripts. Source it (`. scripts/pretty.sh`).
# On a terminal each message starts with a coloured badge (dark text on a green,
# cyan, yellow or red background); piped, or with NO_COLOR set, it is a plain
# mark (✔ ▸ ! ✘) instead. FORCE_COLOR=1 forces the colours.
if { [ -t 1 ] || [ -n "$FORCE_COLOR" ]; } && [ -z "$NO_COLOR" ]; then
  C_RESET=$(printf '\033[0m'); C_DIM=$(printf '\033[2m'); C_BOLD=$(printf '\033[1m')
  C_RED=$(printf '\033[31m'); C_CYAN=$(printf '\033[36m')
  B_STEP=$(printf '\033[1;30;46m RUN \033[0m')
  B_OK=$(printf '\033[1;30;42m OK \033[0m')
  B_WARN=$(printf '\033[1;30;43m WARN \033[0m')
  B_FAIL=$(printf '\033[1;97;41m FAIL \033[0m')
  B_BUILD=$(printf '\033[1;30;46m BUILD \033[0m')
else
  C_RESET=; C_DIM=; C_BOLD=; C_RED=; C_CYAN=
  B_STEP='▸'; B_OK='✔'; B_WARN='!'; B_FAIL='✘'; B_BUILD='▸'
fi
say_step() { printf '%s %s\n' "$B_STEP" "$*"; }
say_ok()   { printf '%s %s\n' "$B_OK" "$*"; }
say_warn() { printf '%s %s\n' "$B_WARN" "$*"; }
say_fail() { printf '%s %s%s%s\n' "$B_FAIL" "$C_BOLD$C_RED" "$*" "$C_RESET"; }
say_dim()  { printf '%s%s%s\n' "$C_DIM" "$*" "$C_RESET"; }
