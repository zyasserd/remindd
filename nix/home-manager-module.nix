{ config, lib, pkgs, ... }:

let
  cfg = config.programs.remindd;

  # Config file location is defined by remindd itself:
  # - $XDG_CONFIG_HOME/remindd/config.yaml (fallback: ~/.config/remindd/config.yaml)
  # We write to that default location and do not set REMINDD_CONFIG.
  configFilePath = "remindd/config.yaml";

  yamlFormat = pkgs.formats.yaml { };

  checkServiceName = "remindd-check";

  # Parse durations like: 30s, 5m, 1h, 2d, 1w, 1h30m.
  # Returns integer seconds or null if invalid.
  parseDurationSeconds = s:
    let
      s' = lib.strings.trim (toString s);
      step = rest:
        let
          m = builtins.match "^([0-9]+)([smhdw])(.*)$" rest;
        in
        if m == null then null else {
          n = lib.toInt (builtins.elemAt m 0);
          unit = builtins.elemAt m 1;
          tail = builtins.elemAt m 2;
        };
      mult = unit:
        if unit == "s" then 1
        else if unit == "m" then 60
        else if unit == "h" then 60 * 60
        else if unit == "d" then 24 * 60 * 60
        else if unit == "w" then 7 * 24 * 60 * 60
        else null;
      go = rest: acc:
        if rest == "" then acc
        else
          let
            p = step rest;
            mlt = if p == null then null else mult p.unit;
          in
          if p == null || mlt == null then null else go p.tail (acc + (p.n * mlt));
    in
    if s' == "" then null else go s' 0;

  # Derive the check interval from settings:
  # - for condition reminders: use check.interval
  # - for interval reminders: use interval
  # We pick the smallest parsed duration across all reminders.
  derivedCheckInterval =
    let
      reminders = cfg.settings.reminders;
      candidates = lib.flatten (lib.mapAttrsToList (_: r:
        if r.type == "condition" then [ r.check.interval ]
        else [ r.interval ]
      ) reminders);
      secs = lib.filter (x: x != null) (map parseDurationSeconds candidates);
      minSecs = if secs == [ ] then null else lib.lists.foldl' builtins.min (builtins.head secs) (builtins.tail secs);
    in
    if minSecs == null then null else "${toString minSecs}s";

  # A small initial delay after login/activation.
  onBootSec = "2m";

  # Typed schema for settings (this is the remindd config, expressed as Nix).
  notificationWindowType = lib.types.submodule ({ ... }: {
    options = {
      start = lib.mkOption { type = lib.types.str; description = "HH:MM"; };
      end = lib.mkOption { type = lib.types.str; description = "HH:MM"; };
    };
  });

  snoozeType = lib.types.submodule ({ ... }: {
    options.default = lib.mkOption { type = lib.types.str; description = "Default snooze duration (e.g. 10m, 2h, 1d)."; };
  });

  actionType = lib.types.submodule ({ ... }: {
    options = {
      label = lib.mkOption { type = lib.types.nullOr lib.types.str; default = null; };
      command = lib.mkOption { type = lib.types.str; description = "Shell command to run for the action."; };
    };
  });

  lastDoneType = lib.types.submodule ({ ... }: {
    options.command = lib.mkOption {
      type = lib.types.str;
      description = "Shell command that prints unix seconds for last done.";
    };
  });

  checkType = lib.types.submodule ({ ... }: {
    options = {
      interval = lib.mkOption { type = lib.types.str; description = "How often to run the check command."; };
      command = lib.mkOption { type = lib.types.str; description = "Shell command; exit 0 means true."; };
    };
  });

  triggerType = lib.types.submodule ({ ... }: {
    options.consecutive = lib.mkOption { type = lib.types.ints.positive; description = "Number of consecutive trues required."; };
  });

  reminderType = lib.types.submodule ({ config, ... }: {
    options = {
      type = lib.mkOption {
        type = lib.types.enum [ "interval" "condition" ];
        description = "Reminder type.";
      };
      label = lib.mkOption { type = lib.types.str; description = "Notification title."; };

      # interval reminders
      interval = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Interval duration (required when type=interval).";
      };
      lastDone = lib.mkOption {
        type = lib.types.nullOr lastDoneType;
        default = null;
        description = "Optional; if omitted, remindd uses per-reminder state.lastDone.";
      };

      # condition reminders
      check = lib.mkOption {
        type = lib.types.nullOr checkType;
        default = null;
        description = "Condition check config (required when type=condition).";
      };
      trigger = lib.mkOption {
        type = lib.types.nullOr triggerType;
        default = null;
        description = "Condition trigger config (required when type=condition).";
      };

      # optional extras
      snooze = lib.mkOption { type = lib.types.nullOr snoozeType; default = null; };
      action = lib.mkOption { type = lib.types.nullOr actionType; default = null; };
    };

    config.assertions = [
      {
        assertion = (config.type == "interval") -> (config.interval != null);
        message = "remindd reminder: interval is required when type=interval";
      }
      {
        assertion = (config.type == "condition") -> (config.check != null && config.trigger != null);
        message = "remindd reminder: check and trigger are required when type=condition";
      }
    ];
  });

  settingsType = lib.types.submodule ({ ... }: {
    options = {
      notificationWindow = lib.mkOption {
        type = lib.types.nullOr notificationWindowType;
        default = null;
      };
      reminders = lib.mkOption {
        type = lib.types.attrsOf reminderType;
        default = { };
      };
    };
  });

in
{
  options.programs.remindd = {
    enable = lib.mkEnableOption "remindd";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.remindd;
      description = "The remindd package to install.";
    };

    # Configuration for remindd, expressed as Nix.
    # This module writes it to $XDG_CONFIG_HOME/${configFilePath}.
    settings = lib.mkOption {
      type = settingsType;
      default = { };
      description = "remindd config as a typed Nix attribute set (written as YAML).";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    assertions = [
      {
        assertion = cfg.settings != { };
        message = "programs.remindd.settings must be set (this module only supports settings-driven config)";
      }
      {
        assertion = cfg.settings.reminders != { };
        message = "programs.remindd.settings.reminders must not be empty";
      }
      {
        assertion = derivedCheckInterval != null;
        message = "could not derive timer interval from settings (use simple durations like 30s/5m/1h/2d/1w or combinations like 1h30m)";
      }
    ];

    # Write config to the default location remindd expects.
    xdg.configFile."${configFilePath}" = {
      source = yamlFormat.generate "remindd-config.yaml" cfg.settings;
    };

    # Automatically run `remindd check` on a user timer.
    systemd.user.services."${checkServiceName}" = {
      Unit = {
        Description = "remindd: evaluate reminders";
      };
      Service = {
        Type = "oneshot";
        ExecStart = "${cfg.package}/bin/remindd check";
      };
    };

    systemd.user.timers."${checkServiceName}" = {
      Unit = {
        Description = "remindd: periodic check";
      };
      Timer = {
        OnBootSec = onBootSec;
        OnUnitActiveSec = derivedCheckInterval;
        Unit = "${checkServiceName}.service";
        Persistent = true;
      };
      Install = {
        WantedBy = [ "timers.target" ];
      };
    };
  };
}
