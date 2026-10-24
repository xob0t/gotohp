{
  description = "gotohp: unofficial Google Photos client (GUI and CLI)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;

      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # vendorHash / npmDepsHash live in nix/hashes.json and are refreshed with
      # nix/update-hashes.sh (see nix/README.md).
      hashes = builtins.fromJSON (builtins.readFile ./nix/hashes.json);

      # Same file the binary embeds (core.GetVersion), so the package version
      # can never drift from what `gotohp` reports.
      version = (builtins.fromJSON (builtins.readFile ./build/windows/info.json)).fixed.file_version;

      src = lib.fileset.toSource {
        root = ./.;
        fileset = lib.fileset.unions [
          ./go.mod
          ./go.sum
          ./main.go
          ./main_cli.go
          ./version.go
          ./core
          ./generated
          ./internal
          ./build/windows/info.json
          ./build/appicon.png
        ];
      };

      mkPackages =
        pkgs:
        let
          # go.mod asks for Go 1.27; fall back to the default Go if nixpkgs has no go_1_27 yet.
          buildGoModule = pkgs.buildGoModule.override { go = pkgs.go_1_27 or pkgs.go; };

          common = {
            inherit version src;
            vendorHash = hashes.vendorHash;
            ldflags = [
              "-s"
              "-w"
            ];
            # Tests touch the network and the user's config dir; run them in CI instead.
            doCheck = false;
          };

          meta = {
            description = "Unofficial Google Photos client";
            homepage = "https://github.com/xob0t/gotohp";
            license = lib.licenses.mit;
            platforms = lib.platforms.linux;
          };
        in
        rec {
          # Vue frontend, embedded into the GUI binary (main.go: //go:embed all:frontend/dist).
          # Wails bindings are committed in frontend/bindings, so wails3 is not needed here.
          frontend = pkgs.buildNpmPackage {
            pname = "gotohp-frontend";
            inherit version;
            src = ./frontend;
            nodejs = pkgs.nodejs_24; # frontend/.node-version
            npmDepsHash = hashes.npmDepsHash;
            installPhase = ''
              runHook preInstall
              cp -r dist $out
              runHook postInstall
            '';
          };

          # Wails v3 GUI (GTK4 + WebKitGTK 6.0). Also accepts the CLI subcommands.
          gotohp = buildGoModule (
            common
            // {
              pname = "gotohp";
              tags = [ "production" ];

              nativeBuildInputs = [
                pkgs.pkg-config
                pkgs.wrapGAppsHook4
                pkgs.copyDesktopItems
              ];
              buildInputs = [
                pkgs.gtk4
                pkgs.webkitgtk_6_0
                pkgs.libsoup_3
                pkgs.glib-networking # TLS for WebKit
              ];

              preBuild = ''
                mkdir -p frontend
                cp -r ${frontend} frontend/dist
                chmod -R u+w frontend/dist
              '';

              postInstall = ''
                mv $out/bin/app $out/bin/gotohp
                install -Dm444 build/appicon.png $out/share/icons/hicolor/128x128/apps/gotohp.png
              '';

              # Wails registers the GTK app id as org.wails.<sanitised app name>
              # ("com_xob0t_gotohp"); matching it lets Wayland compositors find the icon.
              desktopItems = [
                (pkgs.makeDesktopItem {
                  name = "org.wails.com_xob0t_gotohp";
                  desktopName = "gotohp";
                  comment = "Google Photos Unofficial Client";
                  exec = "gotohp";
                  icon = "gotohp";
                  categories = [ "Utility" ];
                  startupWMClass = "org.wails.com_xob0t_gotohp";
                })
              ];

              meta = meta // {
                mainProgram = "gotohp";
              };
            }
          );

          # CLI only: built with -tags cli, no Wails / WebKit / cgo.
          gotohp-cli = buildGoModule (
            common
            // {
              pname = "gotohp-cli";
              tags = [ "cli" ];
              env.CGO_ENABLED = "0";

              postInstall = ''
                mv $out/bin/app $out/bin/gotohp-cli
              '';

              meta = meta // {
                mainProgram = "gotohp-cli";
              };
            }
          );

          default = gotohp;
        };
    in
    {
      packages = forAllSystems mkPackages;

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.gotohp;
        };
        cli = {
          type = "app";
          program = lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.gotohp-cli;
        };
      });

      overlays.default = final: _prev: {
        inherit (mkPackages final) gotohp gotohp-cli;
      };

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          inputsFrom = [ self.packages.${pkgs.stdenv.hostPlatform.system}.gotohp ];
          packages = [
            (pkgs.go_1_27 or pkgs.go)
            pkgs.nodejs_24
            pkgs.go-task
            pkgs.gofumpt
            pkgs.golangci-lint
          ];
        };
      });

      checks = forAllSystems (pkgs: {
        inherit (self.packages.${pkgs.stdenv.hostPlatform.system}) gotohp gotohp-cli frontend;
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt-rfc-style);
    };
}
