#!/bin/sh
# say.sh step|ok|warn|fail|dim MESSAGE...  (for Makefile recipes)
. "$(dirname "$0")/pretty.sh"
kind=$1; shift
case "$kind" in
  step) say_step "$@" ;; ok) say_ok "$@" ;; warn) say_warn "$@" ;; fail) say_fail "$@" ;; dim) say_dim "$@" ;;
  *) echo "say.sh: unknown kind $kind" >&2; exit 2 ;;
esac
