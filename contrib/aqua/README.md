# aqua / mise

Install TerraDrift from GitHub Release archives and verify `checksums.txt`. Linux and macOS only (same as `install.sh`). Does not replace Homebrew or `scripts/install.sh`.

```bash
# From this directory, or copy registry.yaml + aqua.yaml into your infra repo:
aqua i -l
```

mise uses aqua’s package registry:

```bash
mise use aqua:github.com/niravraychura/terradrift@v1.1.1
```

That mise line works after [aquaproj/aqua-registry](https://github.com/aquaproj/aqua-registry) lists `niravraychura/terradrift`. Until then, use the local registry above (`aqua i -l`) or Homebrew / `install.sh`.

## After each `v*` tag

Once the package is in the standard aqua-registry, their updater follows new GitHub Releases — no bump in this repo.

Until that listing exists (or for a private local registry): set the version in `aqua.yaml` to the published tag after `release.yml` has uploaded archives. Do not point at a tag with no GitHub Release assets yet.

To propose the standard-registry listing, copy `registry.yaml` to `pkgs/niravraychura/terradrift/registry.yaml` in aquaproj/aqua-registry.
