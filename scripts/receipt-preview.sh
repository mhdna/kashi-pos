#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
output=${1:-"$repo_dir/receipt-preview.pdf"}

cd "$repo_dir"
go run . --receipt-preview --output "$output"
