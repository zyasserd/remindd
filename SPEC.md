# remindd v1 — Specification (Implementation Handoff)

## 1. Overview

`remindd` is a desktop reminder system.

**Key properties**
- Stateless execution: invoked on-demand; no daemon.
- Declarative configuration: reminders defined in a single YAML file.
- Minimal persistent state: per-reminder YAML state file.
- Desktop notifications: Linux via `notify-send`.
- Extensible reminder types: `interval` and `condition`.

## 2. Terms

- **Reminder**: named config entry that may become due.
- **Check cycle**: one execution of `remindd check`.
- **Now**: the current wall-clock time (local timezone) at time of evaluation.
- **Reference time**: not separately defined in v1; interval reminders use `lastDone.command` as their last-done reference.
- **Notification window**: global time-of-day range (local time) during which notifications are allowed.
- **Action**: optional shell command to run when user triggers it.

## 3. Configuration

### 3.1 Location

- Default: `$XDG_CONFIG_HOME/remindd/config.yaml`
- If `XDG_CONFIG_HOME` is unset/empty, default to `$HOME/.config`.
- Override: environment variable `REMINDD_CONFIG` (absolute or relative path).

### 3.2 YAML schema

```yaml
notificationWindow:
  start: "18:00"   # inclusive, 24-hour format, local time
  end: "22:00"     # exclusive, 24-hour format, local time

reminders:
  <name>:
    type: interval | condition
    label: string
    action: # optional
      label: string         # optional; if omitted, use "Run"
      command: string
    snooze:
      default: <duration>  # optional; default="1d"

    # interval reminders only
    interval: <duration>
    lastDone:
      command: string       # runs in shell; stdout is unix timestamp

    # condition reminders only
    check:
      interval: <duration>  # minimum interval between evaluations
      command: string       # exit code 0=true; non-zero=false
    trigger:
      consecutive: integer  # >= 1

```

### 3.3 Field constraints and parsing

**Reminder name (`<name>`)**
- YAML map key.
- Must be non-empty.
- Must be safe for filenames; v1 constraint: `^[A-Za-z0-9_-]+$`.

**Type**
- Required. Must be exactly `interval` or `condition`.

**Label**
- Required. Non-empty.

**Duration format (v1)**
- Accept Go `time.ParseDuration` formats (e.g. `"30m"`, `"1h"`) **plus**:
  - `Nd` meaning N * 24h
  - `Nw` meaning N * 7d
- Integers only for `d` and `w` suffixes (no decimals).
- `check.interval`, `interval`, `snooze.default` must be > 0.

**Commands**
- Executed via the system shell (`/bin/sh -c <command>`).
- The tool must not interpret stdout except where specified.

**Interval reminder requirements**
- Required: `interval`.
- Optional: `lastDone.command`.
- If `lastDone.command` is omitted or empty, `remindd` uses the persisted per-reminder state field `lastDone` as the last-done reference time (missing/null => `0`).
- Forbidden/ignored: `check`, `trigger`.

**Condition reminder requirements**
- Required: `check.interval`, `check.command`, `trigger.consecutive`.
- `trigger.consecutive` must be >= 1.
- Forbidden/ignored: `interval`, `lastDone`.

**Snooze**
- `snooze.default` optional; default `1d`.

**Notification window**
- Optional. If missing, notifications are allowed at any time.
- If present, both `start` and `end` are required.
- Times parsed as `HH:MM` (00:00–23:59), local time.
- Window semantics:
  - If `start < end`: allow `[start, end)`.
  - If `start > end`: allow wrap-around `[start, 24:00) ∪ [00:00, end)`.
  - If `start == end`: allow all day (treat as no restriction).

## 4. Persistent State

### 4.1 Location

Per reminder:

```
$XDG_STATE_HOME/remindd/state/<name>.yaml
```

- If `XDG_STATE_HOME` is unset/empty, default to `$HOME/.local/state`.
- The directory must be created if missing.

### 4.2 Schema (per reminder)

```yaml
lastDone: <unix timestamp> | null
trueStreak: integer
firstTrueAt: <unix timestamp> | null
snoozedUntil: <unix timestamp> | null
lastNotifiedAt: <unix timestamp> | null
lastCheckAt: <unix timestamp> | null   # used for condition check.interval gating
```

**Notes**
- Missing file means empty state (all fields null/zero).
- Unused fields for a reminder type may be omitted.

### 4.3 Rules

- Atomic writes: write to temp file in same directory, `fsync`, then rename.
- Only `remindd` mutates state.

## 5. CLI

### 5.1 Commands

- `remindd check`
  - Evaluate all reminders; send notifications for due reminders; update state.

- `remindd run <name>`
  - Execute reminder action (if present). On success:
    - interval: set `lastDone=now` in state.
    - condition: reset `trueStreak=0`, `firstTrueAt=null`.
    - clear `snoozedUntil`.

- `remindd snooze <name> <duration>`
  - Set `snoozedUntil = now + duration`.

- `remindd list`
  - Print all reminders and key state and whether due/overdue.

### 5.2 Exit codes

- `0`: success
- `1`: config error (invalid config, parse error, missing required fields)
- `2`: state I/O error (read/write/rename failures)
- `3`: action failed (action command non-zero or spawn failure)
- `4`: unknown reminder name

## 6. Evaluation Semantics

### 6.1 Common pre-checks (per reminder)

1. Load per-reminder state (or empty).
2. If `snoozedUntil != null` and `now < snoozedUntil`: skip evaluation and do not notify.
3. If notification window is configured and `now` is outside it: skip evaluation and do not notify.
4. Proceed to type-specific evaluation.

### 6.2 Interval reminders

1. Determine `lastDone` (unix seconds):
  - If `lastDone.command` is present and non-empty: execute it and parse stdout as a Unix timestamp.
    - Ignore surrounding whitespace.
    - On parse failure: treat as config error.
  - Otherwise: read `lastDone` from the per-reminder state file (missing/null => `0`).
3. Compute:
   - `dueAt = lastDone + interval`
   - `overdue = now - dueAt`
4. Due if `overdue > 0`.
5. Body text: `Overdue by X` where X is a human readable duration rounded down to minutes, but if >= 24h show whole days.

### 6.3 Condition reminders

1. If `lastCheckAt != null` and `now - lastCheckAt < check.interval`: skip running command; do not change streak; not due.
2. Run `check.command`.
   - exit code `0` => true
   - any other exit code => false
   - spawn failure => treat as config error.
3. Update state:
   - Always set `lastCheckAt=now`.
   - If true:
     - `trueStreak += 1`
     - if `firstTrueAt` is null, set to `now`.
   - If false:
     - `trueStreak = 0`
     - `firstTrueAt = null`
4. Due if `trueStreak >= trigger.consecutive`.
5. Body text: `Condition true for <trueStreak> consecutive checks`.

### 6.4 Snooze

- Snoozed reminders do not notify.
- `remindd snooze` always overwrites any existing snooze.

### 6.5 Rate limiting within a check cycle

- A reminder may produce at most one notification per `remindd check` execution.
- Persist `lastNotifiedAt=now` when a notification is sent.
- v1 does **not** suppress notifications across multiple runs based solely on `lastNotifiedAt` (only used for display/auditing), except the “at most one per cycle” rule.

## 7. Notifications

- Use `notify-send`.
- Title is the reminder `label`.
- Body is computed per reminder type.
- Actions:
  - If `action.command` exists, provide an action button labeled `action.label` (or `Run`) that triggers `remindd run <name>`.
  - Always provide a snooze action button that triggers `remindd snooze <name> <defaultDuration>`.

**Execution model for actions**
- Notifications invoke `remindd` commands, not arbitrary commands directly.
- `remindd run` and `remindd snooze` are responsible for state changes.

## 8. Guarantees

- `check` is idempotent with respect to a single execution (no duplicate notification per reminder).
- Overdue is computed dynamically; missed executions do not break schedule.
- Single source of truth: one state file per reminder.

## 9. Example config

```yaml
notificationWindow:
  start: "18:00"
  end: "22:00"

reminders:
  systemUpdates:
    type: interval
    label: "Update your NixOS system"
    interval: 3w
    lastDone:
      command: stat -c %Y /etc/nixos/flake.lock || echo 0
    action:
      label: "Run update"
      command: "nixos-rebuild switch"
    snooze:
      default: 1d

  backups:
    type: interval
    label: "Run backups"
    interval: 7d
    lastDone:
      command: kopia last-snapshot || echo 0
    action:
      command: "kopia snapshot create"

  powerSaver:
    type: condition
    label: "Power saver off too long"
    check:
      interval: 30m
      command: "powerprofilesctl get | grep -vq power-saver"
    trigger:
      consecutive: 12
```
