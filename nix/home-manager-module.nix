{ config, lib, pkgs, ... }:

let
  cfg = config.programs.remindd;

  configFilePath = "remindd/config.yaml";

  mkEnv = [
    "XDG_CONFIG_HOME=%h/.config"
    "XDG_STATE_HOME=%h/.local/state"
  ];

  checkServiceName = "remindd-check";

in
{
  options.programs.remindd = {
    enable = lib.mkEnableOption "remindd";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.remindd;
      description = "The remindd package to install.";
    };

    configText = lib.mkOption {
      type = lib.types.nullOr lib.types.lines;
      default = null;
      description = "If set, writes $XDG_CONFIG_HOME/${configFilePath}.";
    };

    checkInterval = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "15m";
      description = "If set, enables a user systemd timer to run `remindd check` at this interval (e.g. 5m, 1h).";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    xdg.configFile."${configFilePath}" = lib.mkIf (cfg.configText != null) {
      text = cfg.configText;
    };

    systemd.user.services."${checkServiceName}" = lib.mkIf (cfg.checkInterval != null) {
      Unit = {
        Description = "remindd: evaluate reminders";
      };
      Service = {
        Type = "oneshot";
        Environment = mkEnv;
        ExecStart = "${cfg.package}/bin/remindd check";
      };
    };

    systemd.user.timers."${checkServiceName}" = lib.mkIf (cfg.checkInterval != null) {
      Unit = {
        Description = "remindd: periodic check";
      };
      Timer = {
        OnBootSec = "2m";
        OnUnitActiveSec = cfg.checkInterval;
        Unit = "${checkServiceName}.service";
        Persistent = true;
      };
      Install = {
        WantedBy = [ "timers.target" ];
      };
    };
  };
}
