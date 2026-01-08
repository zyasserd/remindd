{ config, lib, pkgs, remindd, ... }:

let
  cfg = config.services.remindd;

  # Config file location is defined by remindd itself:
  # - $XDG_CONFIG_HOME/remindd/config.yaml (fallback: ~/.config/remindd/config.yaml)
  # We write to that default location and do not set REMINDD_CONFIG.
  configFilePath = "remindd/config.yaml";

  yamlFormat = pkgs.formats.yaml { };

  checkServiceName = "remindd-check";

  # Derive the check interval from settings:
  # - use reminder.every for all reminders
  # We pick the smallest interval across all reminders.
  derivedCheckInterval =
    let
      reminders = cfg.reminders;
      secs = lib.flatten (lib.mapAttrsToList (_: r: [ r.every ]) reminders);
      minSecs = if secs == [ ] then null else lib.lists.foldl' lib.min (builtins.head secs) (builtins.tail secs);
    in
    if minSecs == null then null else "${toString minSecs}s";

  # A small initial delay after login/activation.
  onBootSec = "2m";

  # Typed schema for settings (this is the remindd config, expressed as Nix).
  notifyWindowType = lib.types.submodule ({ ... }: {
    options = {
      from = lib.mkOption { type = lib.types.str; description = "HH:MM"; };
      to = lib.mkOption { type = lib.types.str; description = "HH:MM"; };
    };
  });

  actionType = lib.types.submodule ({ ... }: {
    options = {
      label = lib.mkOption { type = lib.types.nullOr lib.types.str; default = null; };
      command = lib.mkOption { type = lib.types.str; description = "Shell command to run for the action."; };
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

      every = lib.mkOption {
        type = lib.types.ints.positive;
        description = "How often to execute (seconds).";
      };

      lastDoneOverride = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Optional; shell command that prints unix seconds for last done (overrides state.lastDone).";
      };

      conditionCommand = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Condition reminders only; shell command where exit 0 means true.";
      };

      trigger = lib.mkOption {
        type = lib.types.nullOr lib.types.ints.positive;
        default = null;
        description = "Condition reminders only; number of consecutive trues required.";
      };
    };
  });

  settings = {
    notifyWindow = cfg.notifyWindow;
    reminders = cfg.reminders;
  };

  mkAssert = assertion: message: { inherit assertion message; };

  mkReminderAssertions = reminders:
    lib.flatten (lib.mapAttrsToList (name: r:
      let
        prefix = "services.remindd.reminders.${name}";
      in
      [
        (mkAssert ((r.type != "condition") || (r.conditionCommand != null))
          "${prefix}: conditionCommand is required when type=condition")
        (mkAssert ((r.type != "condition") || (r.trigger != null))
          "${prefix}: trigger is required when type=condition")
        (mkAssert ((r.type == "condition") || (r.conditionCommand == null))
          "${prefix}: conditionCommand is only allowed when type=condition")
        (mkAssert ((r.type == "condition") || (r.trigger == null))
          "${prefix}: trigger is only allowed when type=condition")
      ]
    ) reminders);

in
{
  options.services.remindd = {
    enable = lib.mkEnableOption "remindd";

    notifyWindow = lib.mkOption {
      type = lib.types.nullOr notifyWindowType;
      default = null;
      description = "Optional notification window (HH:MM local time).";
    };

    reminders = lib.mkOption {
      type = lib.types.attrsOf reminderType;
      default = { };
      description = "remindd reminders as a typed Nix attribute set.";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [
      remindd
      pkgs.libnotify
    ];

    assertions =
      [
        (mkAssert (cfg.reminders != { }) "services.remindd.reminders must not be empty")
      ]
      ++ (mkReminderAssertions cfg.reminders);

    # Write config to the default location remindd expects. 
    xdg.configFile."${configFilePath}" = {
      source = yamlFormat.generate "remindd-config.yaml" settings;
    };

    # Automatically run `remindd check` on a user timer.
    systemd.user.services."${checkServiceName}" = {
      Unit = {
        Description = "remindd: evaluate reminders";
      };
      Service = {
        Type = "oneshot";
        ExecStart = "${remindd}/bin/remindd check";
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
