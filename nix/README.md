# Nix

A flake for Linux (`x86_64` and `aarch64`). It is community-maintained: the upstream author does not use Nix, so it may lag behind or break when dependencies change. Fixes are welcome as PRs.

## Packages

- `gotohp`: the GUI (GTK4 + WebKitGTK 6.0). It also accepts the CLI commands.
- `gotohp-cli`: the standalone CLI, with no GUI dependencies.
- `frontend`: the Vue frontend that gets embedded into the GUI.

## Install

Try it without installing:

```shell
nix run github:xob0t/gotohp#gotohp                    # GUI
nix run github:xob0t/gotohp#gotohp-cli -- --help      # CLI
```

Install into your profile:

```shell
nix profile install github:xob0t/gotohp#gotohp github:xob0t/gotohp#gotohp-cli
```

Or on NixOS, add it as a flake input:

```nix
# flake.nix
inputs.gotohp = {
  url = "github:xob0t/gotohp";
  inputs.nixpkgs.follows = "nixpkgs";
};

# in your configuration
environment.systemPackages = [
  inputs.gotohp.packages.${pkgs.stdenv.hostPlatform.system}.gotohp
  inputs.gotohp.packages.${pkgs.stdenv.hostPlatform.system}.gotohp-cli
];
```

For development, `nix develop` provides Go, Node.js and the GTK/WebKit libraries.

## Maintenance

The package version is read from `build/windows/info.json`, the same file the binary embeds, so it never needs bumping here.

The Go and npm dependency hashes live in `nix/hashes.json`. After changing `go.mod`, `go.sum` or `frontend/package-lock.json`, refresh them:

```shell
nix/update-hashes.sh
```

To update nixpkgs, run `nix flake update`. Check the result with:

```shell
nix flake check
nix build .#gotohp .#gotohp-cli
```

Flakes only see git-tracked files, so `git add` any new files first.
