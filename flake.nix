{
  description = "remindd: stateless desktop reminders";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, utils }:
    let
      hmModule = import ./nix/home-manager.nix;
    in
    {
      homeManagerModules.remindd = { pkgs, ... }:
        {
          imports = [ hmModule ];
          _module.args.remindd = self.packages.${pkgs.system}.remindd;
        };
      homeManagerModules.default = self.homeManagerModules.remindd;
    }
    // utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        remindd = pkgs.buildGoModule {
          pname = "remindd";
          version = "0.1.0";

          src = ../.;

          vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";

          subPackages = [ "cmd/remindd" ];
          ldflags = [ "-s" "-w" ];

          doCheck = true;
        };
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
