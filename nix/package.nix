{ buildGoModule }:

buildGoModule {
  pname = "remindd";
  version = "0.1.0";

  src = ../.;

  vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";

  subPackages = [ "cmd/remindd" ];
  ldflags = [ "-s" "-w" ];

  doCheck = true;
}
