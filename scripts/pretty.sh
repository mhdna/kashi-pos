# Coloured one-line messages for the release scripts. Source it (`. scripts/pretty.sh`).
# Colours only on a terminal, never with NO_COLOR set; FORCE_COLOR=1 forces them.
if { [ -t 1 ] || [ -n "$FORCE_COLOR" ]; } && [ -z "$NO_COLOR" ]; then
  C_RESET=$(printf '\033[0m'); C_DIM=$(printf '\033[2m'); C_BOLD=$(printf '\033[1m')
  C_RED=$(printf '\033[31m'); C_GREEN=$(printf '\033[32m'); C_YELLOW=$(printf '\033[33m'); C_CYAN=$(printf '\033[36m')
else
  C_RESET=; C_DIM=; C_BOLD=; C_RED=; C_GREEN=; C_YELLOW=; C_CYAN=
fi
say_step() { printf '%s▸%s %s\n' "$C_CYAN" "$C_RESET" "$*"; }
say_ok()   { printf '%s✔%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
say_warn() { printf '%s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*"; }
say_fail() { printf '%s✘%s %s%s%s\n' "$C_RED" "$C_RESET" "$C_BOLD" "$*" "$C_RESET"; }
say_dim()  { printf '%s%s%s\n' "$C_DIM" "$*" "$C_RESET"; }
