#!/usr/bin/env zsh
set -euo pipefail

repo_root="${0:A:h:h}"
cd "$repo_root"

echo "[smoke] building"
go build ./cmd/remindd

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

export XDG_CONFIG_HOME="$workdir/config"
export XDG_STATE_HOME="$workdir/state"
export REMINDD_CONFIG="$repo_root/examples/config.yaml"

mkdir -p "$XDG_CONFIG_HOME" "$XDG_STATE_HOME"

echo "[smoke] list"
./remindd list

echo "[smoke] snooze demoInterval 600"
./remindd snooze demoInterval 600
./remindd list

echo "[smoke] run demoInterval"
./remindd run demoInterval
./remindd list

if command -v notify-send >/dev/null 2>&1; then
  echo ""
  echo "[smoke] optional: check (will display a notification; may block until dismissed if notify-send supports --wait)"
  echo -n "Run check now? [y/N] "
  read -r ans
  if [[ "$ans" == "y" || "$ans" == "Y" ]]; then
    ./remindd check
  fi
else
  echo "[smoke] notify-send not found; skipping notification check"
fi

echo "[smoke] done (temp state in $workdir)"
