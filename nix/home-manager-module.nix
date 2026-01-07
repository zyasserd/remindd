{ config, lib, pkgs, ... }:

let
  cfg = config.programs.remindd;

  # Config file location is defined by remindd itself:
  # - $XDG_CONFIG_HOME/remindd/config.yaml (fallback: ~/.config/remindd/config.yaml)
  # We write to that default location and do not set REMINDD_CONFIG.
  configFilePath = "remindd/config.yaml";

  yamlFormat = pkgs.formats.yaml { };

  checkServiceName = "remindd-check";

  # Derive the check interval from settings:
  # - for condition reminders: use condition.interval
  # - for interval reminders: use interval.duration
  # We pick the smallest interval across all reminders.
  derivedCheckInterval =
    let
      reminders = cfg.settings.reminders;
      secs = lib.flatten (lib.mapAttrsToList (_: r:
        if r.type == "condition" && r.condition != null then [ r.condition.interval ]
        else if r.type == "interval" && r.interval != null then [ r.interval.duration ]
        else [ ]
      ) reminders);
      minSecs = if secs == [ ] then null else lib.lists.foldl' lib.min (builtins.head secs) (builtins.tail secs);
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

  actionType = lib.types.submodule ({ ... }: {
    options = {
      label = lib.mkOption { type = lib.types.nullOr lib.types.str; default = null; };
      command = lib.mkOption { type = lib.types.str; description = "Shell command to run for the action."; };
    };
  });

  intervalType = lib.types.submodule ({ ... }: {
    options = {
      duration = lib.mkOption {
        type = lib.types.ints.positive;
        description = "Interval duration in seconds.";
      };
      lastDoneCommand = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Optional; shell command that prints unix seconds for last done.";
      };
    };
  });

  conditionType = lib.types.submodule ({ ... }: {
    options = {
      interval = lib.mkOption {
        type = lib.types.ints.positive;
        description = "How often to run the condition check (seconds).";
      };
      command = lib.mkOption {
        type = lib.types.str;
        description = "Shell command; exit 0 means true.";
      };
      trigger = lib.mkOption {
        type = lib.types.ints.positive;
        description = "Number of consecutive trues required.";
      };
    };
  });

  reminderType = lib.types.submodule ({ ... }: {
    options = {
      type = lib.mkOption {
        type = lib.types.enum [ "interval" "condition" ];
        description = "Reminder type.";
      };
      label = lib.mkOption { type = lib.types.str; description = "Notification title."; };

      # optional extras
      snooze = lib.mkOption {
        type = lib.types.nullOr lib.types.ints.positive;
        default = null;
        description = "Optional; snooze duration in seconds (default handled by remindd).";
      };
      action = lib.mkOption { type = lib.types.nullOr actionType; default = null; };

      # interval reminders
      interval = lib.mkOption {
        type = lib.types.nullOr intervalType;
        default = null;
        description = "Interval config (required when type=interval).";
      };

      # condition reminders
      condition = lib.mkOption {
        type = lib.types.nullOr conditionType;
        default = null;
        description = "Condition config (required when type=condition).";
      };
    };
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
      description = "remindd config as a typed Nix attribute set.";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [
      cfg.package
      pkgs.libnotify
    ];

    assertions = [
      {
        assertion = cfg.settings != { };
        message = "programs.remindd.settings must be set (this module only supports settings-driven config)";
      }
      {
        assertion = cfg.settings.reminders != { };
        message = "programs.remindd.settings.reminders must not be empty";
      }
    ]
    ++ (lib.mapAttrsToList (name: r: {
      assertion = (r.type == "interval") -> (r.interval != null);
      message = "programs.remindd.settings.reminders.${name}: interval is required when type=interval";
    }) cfg.settings.reminders)
    ++ (lib.mapAttrsToList (name: r: {
      assertion = (r.type == "condition") -> (r.condition != null);
      message = "programs.remindd.settings.reminders.${name}: condition is required when type=condition";
    }) cfg.settings.reminders)
    ++ [
      {
        assertion = derivedCheckInterval != null;
        message = "could not derive timer interval from settings (ensure reminders include interval.duration or condition.interval)";
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
