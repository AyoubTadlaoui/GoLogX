{
  description = "logx — pretty-print JSON slog logs from stdin, files, or follow mode";

  inputs = {
    # Pin to a stable channel. Bump periodically by running `nix flake update`.
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        # Single source of truth for the build's reported version.
        # Keep in sync with the latest git tag — goreleaser bakes this same
        # value into the prebuilt binaries via -ldflags.
        version = "0.1.6";
      in {
        packages = rec {
          logx = pkgs.buildGoModule {
            pname = "logx";
            inherit version;
            src = ./.;
            subPackages = [ "cmd/logx" ];
            # GoLogX has zero external dependencies, so no vendor tree exists
            # and there is no go.sum to hash. buildGoModule treats null as
            # "deps-free build".
            vendorHash = null;
            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];
            doCheck = true;
            meta = with pkgs.lib; {
              description = "Pretty-print JSON slog logs from stdin, files, or follow mode";
              homepage = "https://github.com/AyoubTadlaoui/GoLogX";
              license = licenses.mit;
              maintainers = [{
                name = "Ayoub Tadlaoui";
                email = "atlas.kaisar@icloud.com";
                github = "AyoubTadlaoui";
              }];
              mainProgram = "logx";
              platforms = platforms.unix;
            };
          };
          default = logx;
        };

        apps = rec {
          logx = flake-utils.lib.mkApp { drv = self.packages.${system}.logx; };
          default = logx;
        };

        # Drop into a shell with everything needed to hack on GoLogX.
        # `nix develop` (or direnv `use flake`).
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            goreleaser
            gnumake
          ];
        };

        # `nix flake check` will build this.
        checks.build = self.packages.${system}.logx;
      });
}
