{
  description = "remindd: stateless desktop reminders";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, utils }:
    let
      overlay = import ./nix/overlay.nix;
      hmModule = import ./nix/home-manager-module.nix;
    in
    {
      overlays.default = overlay;
      homeManagerModules.default = hmModule;
      homeManagerModules.remindd = hmModule;
    }
    // utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ overlay ];
        };
        remindd = pkgs.remindd;
      in {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
          ];
        };

        packages = {
          default = remindd;
          inherit remindd;
        };

        apps.default = {
          type = "app";
          program = "${remindd}/bin/remindd";
        };
      }
    );

}
