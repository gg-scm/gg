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

        inherit (pkgs.lib.attrsets) mapAttrs' nameValuePair optionalAttrs;

        mkCheck = git: self.packages.${system}.default.override {
          inherit git;
          doCheck = true;
        };

        gitChecks = mapAttrs' (name: git: nameValuePair ("with_" + name) (mkCheck git)) gitPackages;
      in
      {
        packages = gitPackages // {
          default = pkgs.callPackage ./. {
            buildGoModule = pkgs.buildGo121Module;
            commit = self.sourceInfo.rev or null;
          };
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/gg";
        };

        devShells.default = pkgs.mkShell {
          inputsFrom = [ self.packages.${system}.default ];

          packages = [
            pkgs.git
            pkgs.go-tools # static check
            pkgs.gotools  # stringer, etc.
            pkgs.python3
          ];
        };

        checks = {
          default = mkCheck pkgs.git;
        } // optionalAttrs pkgs.hostPlatform.isLinux gitChecks;
      }
    );
}
