# remindd

Desktop reminders (no daemon) driven by a YAML config.

`remindd` is a CLI you run periodically (cron/systemd timer). It stores per-reminder state in `$XDG_STATE_HOME`.

## Requirements

- `notify-send` available in `PATH` (typically from `libnotify` / your desktop environment)

## Install

### Go

```zsh
go test ./...
go build ./cmd/remindd
```

### Nix

```zsh
nix build .#remindd
nix run .#remindd -- list
```

## Quickstart

Create `~/.config/remindd/config.yaml` (or set `REMINDD_CONFIG`):

```yaml
notificationWindow:
  start: "18:00"
  end: "22:00"

reminders:
  demoInterval:
    type: interval
    label: "Stretch"
    interval: 2h
    # lastDone.command is optional; if omitted, remindd uses per-reminder state.lastDone.
    action:
      label: "Done"
      command: "echo stretched"
```

Run:

```zsh
./remindd list
./remindd check
```

## Commands

- `remindd check`: evaluate reminders and notify if due
- `remindd list`: print reminder status
- `remindd run <name>`: run the reminder action and update state
- `remindd snooze <name> <duration>`: snooze a reminder (e.g. `10m`, `2h`, `3d`, `1w`)

## Config

Default config path:
- `$XDG_CONFIG_HOME/remindd/config.yaml` (fallback `$HOME/.config/remindd/config.yaml`)
- override with `REMINDD_CONFIG`

Examples:
- `examples/config.yaml`
- `examples/config_state_lastdone.yaml`

See `SPEC.md` for the full schema.

### Reminder types

- **interval**: due when `now > lastDone + interval`
	- last done source:
		- `lastDone.command` stdout (unix seconds), if configured
		- otherwise `state.lastDone` (missing/null => `0`)
- **condition**: runs `check.command` every `check.interval`; due after `trigger.consecutive` successes

Commands are executed via `/bin/sh -c`. If you need bash features, wrap with `bash -lc '...'`.

## State

Per-reminder state files:
- `$XDG_STATE_HOME/remindd/state/<name>.yaml` (fallback `$HOME/.local/state/...`)

## Home Manager (Nix)

This flake exports a Home Manager module: `homeManagerModules.remindd` (also available as `homeManagerModules.default`).

Example `home.nix`:

```nix
{ inputs, ... }:
{

	imports = [ inputs.remindd.homeManagerModules.remindd ];

	programs.remindd = {
		enable = true;

		# Required: config as a typed Nix attrset (written to $XDG_CONFIG_HOME/remindd/config.yaml)
		settings = {
			notificationWindow = {
				start = "18:00";
				end = "22:00";
			};
			reminders = { };
		};
	};
}
```

## Example + smoke test

```zsh
chmod +x scripts/smoke.zsh
./scripts/smoke.zsh
```

## Notes

- Notification action buttons require a `notify-send` that supports `--action` and `--wait`; otherwise actions are ignored and `check` won’t wait for a selection.
