# remindd v2 — Specification (Implementation Handoff)

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
- **Reference time**: for interval reminders, the “last done” time comes from either `lastDoneOverride` (if set) or persisted state.
- **Notification window**: global time-of-day range (local time) during which notifications are allowed.
- **Action**: optional shell command to run when user triggers it.

## 3. Configuration

### 3.1 Location

- Default: `$XDG_CONFIG_HOME/remindd/config.yaml`
- If `XDG_CONFIG_HOME` is unset/empty, default to `$HOME/.config`.
- Override: environment variable `REMINDD_CONFIG` (absolute or relative path).

### 3.2 YAML schema

```yaml
notifyWindow:
  # Optional. If omitted, notifications are allowed all day.
  # To allow all day explicitly, set from == to (e.g. "00:00" / "00:00").
  from: "18:00"   # inclusive, 24-hour format, local time
  to: "22:00"     # exclusive, 24-hour format, local time

reminders:
  <name>:
    type: interval | condition
    label: string
    action: # optional
      label: string   # optional; default="Run"
      command: string # required if action is present

    snooze: integer # optional; default=86400 (seconds)

    # Optional. If present and non-empty, stdout is Unix epoch seconds and overrides the persisted `lastDone`.
    lastDoneOverride: string

    # how often to execute (interval) or attempt to execute in case of condition
    every: integer

    # condition reminders only
    conditionCommand: string  # exit code 0=true; non-zero=false
    trigger: integer          # >= 1

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

**Durations (v2)**
- All durations are integers in seconds.
- `every` and `snooze` must be > 0.

**Commands**
- Executed via the system shell (`/bin/sh -c <command>`).
- The tool must not interpret stdout except where specified.

**Interval reminder requirements**
- Required: `every`.
- Optional: `lastDoneOverride`.
- If `lastDoneOverride` is omitted or empty, `remindd` uses the persisted per-reminder state field `lastDone` as the last-done reference time (missing/null => `0`).
- Forbidden/ignored: `conditionCommand`, `trigger`.

**Condition reminder requirements**
- Required: `every`, `conditionCommand`, `trigger`.
- `trigger` must be >= 1.
- `lastDoneOverride` is allowed but ignored for condition evaluation.

**Snooze**
- `snooze` optional; default `86400`.

**Notification window**
- Optional. If missing, notifications are allowed at any time.
- If present, both `from` and `to` are required.
- Times parsed as `HH:MM` (00:00–23:59), local time.
- Window semantics:
  - If `from < to`: allow `[from, to)`.
  - If `from > to`: allow wrap-around `[from, 24:00) ∪ [00:00, to)`.
  - If `from == to`: allow all day (treat as no restriction).

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
lastCheckAt: <unix timestamp> | null   # used for condition every gating
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

- `remindd snooze <name> <seconds>`
  - Set `snoozedUntil = now + seconds`.

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
  - If `lastDoneOverride` is present and non-empty: execute it and parse stdout as a Unix timestamp in seconds since the Unix epoch.
    - Ignore surrounding whitespace.
    - On parse failure: treat as config error.
  - Otherwise: read `lastDone` from the per-reminder state file (missing/null => `0`).
3. Compute:
  - `dueAt = lastDone + every`
   - `overdue = now - dueAt`
4. Due if `overdue > 0`.
5. Body text: `Overdue by X` where X is a human readable duration rounded down to minutes, but if >= 24h show whole days.

### 6.3 Condition reminders

1. If `lastCheckAt != null` and `now - lastCheckAt < every`: skip running command; do not change streak; not due.
2. Run `conditionCommand`.
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
4. Due if `trueStreak >= trigger`.
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
  - Always provide a snooze action button that triggers `remindd snooze <name> <snooze>`.

**Execution model for actions**
- Notifications invoke `remindd` commands, not arbitrary commands directly.
- `remindd run` and `remindd snooze` are responsible for state changes.

## 8. Guarantees

- `check` is idempotent with respect to a single execution (no duplicate notification per reminder).
- Overdue is computed dynamically; missed executions do not break schedule.
- Single source of truth: one state file per reminder.

## 9. Example config

```yaml
notifyWindow:
  from: "18:00"
  to: "22:00"

reminders:
  systemUpdates:
    type: interval
    label: "Update your NixOS system"
    snooze: 86400
    every: 1814400 # 3w
    lastDoneOverride: stat -c %Y /etc/nixos/flake.lock || echo 0
    action:
      label: "Run update"
      command: "nixos-rebuild switch"

  backups:
    type: interval
    label: "Run backups"
    every: 604800 # 7d
    lastDoneOverride: kopia last-snapshot || echo 0
    action:
      command: "kopia snapshot create"

  powerSaver:
    type: condition
    label: "Power saver off too long"
    every: 1800 # 30m
    conditionCommand: "powerprofilesctl get | grep -vq power-saver"
    trigger: 12
```
