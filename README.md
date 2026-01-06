# remindd

Desktop reminders (no daemon) driven by YAML config.

## Build

```zsh
go test ./...

go build ./cmd/remindd
```

## Run

```zsh
./remindd list
./remindd check
./remindd snooze systemUpdates 1d
./remindd run systemUpdates
```

## Example + smoke test

- Example config: `examples/config.yaml`
- Interval reminder using state for lastDone: `examples/config_state_lastdone.yaml`
- Smoke script (uses temp XDG dirs):

```zsh
chmod +x scripts/smoke.zsh
./scripts/smoke.zsh
```

## Config

Default config path:
- `$XDG_CONFIG_HOME/remindd/config.yaml` (fallback `$HOME/.config/remindd/config.yaml`)
- override with `REMINDD_CONFIG`

See `SPEC.md` for the full schema.
