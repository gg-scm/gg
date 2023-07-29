{
  description = "Git with less typing";

  inputs = {
    nixpkgs.url = "nixpkgs";
    gg-git.url = "github:gg-scm/gg-git";
    flake-utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, gg-git, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        gitPackages = (pkgs.lib.attrsets.filterAttrs
          (k: _: pkgs.lib.strings.hasPrefix "git_" k)
          gg-git.packages.${system});
      in
      {
        packages = gitPackages // {
          default = pkgs.callPackage ./. {
            buildGoModule = pkgs.buildGo120Module;
            commit = self.sourceInfo.rev or null;
          };
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/gg";
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.git
            pkgs.go-tools # static check
            pkgs.gotools  # stringer, etc.
            pkgs.python3

            self.packages.${system}.default.go
          ];
        };
      }
    );
}
