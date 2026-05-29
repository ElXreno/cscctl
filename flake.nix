{
  description = "cscctl - Samsung Galaxy CSC changer for Linux";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" ];
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.buildGoModule {
            pname = "cscctl";
            version = "0.1.0"; # x-release-please-version

            src = self;
            subPackages = [ "." ];

            vendorHash = "sha256-KkDKZitPCBlKd5O1LOOT/aE5wHOsY6jznkPSQJ7jVQI=";

            meta.mainProgram = "cscctl";
          };
        });

      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
            ];
          };
        });
    };
}
