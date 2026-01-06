{
  description = "remindd: stateless desktop reminders";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, utils }: utils.lib.eachDefaultSystem (system:
    let
      pkgs = import nixpkgs {
        inherit system;
        # config.allowUnfree = true;
      };
      version = "0.1.0";

      remindd = pkgs.buildGoModule {
        pname = "remindd";
        inherit version;
        src = ./.;

        # If this hash is wrong, `nix build` will tell you the correct one.
        vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";

        subPackages = [ "cmd/remindd" ];
        ldflags = [ "-s" "-w" ];

        # No extra runtime deps; notify-send is optional at runtime.
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
