# Homebrew

Published tap: [`niravraychura/homebrew-terradrift`](https://github.com/niravraychura/homebrew-terradrift).

```bash
brew install niravraychura/terradrift/terradrift
```

That installs the GitHub Release archive for your OS/arch and checks `sha256` from `checksums.txt`. Prefer `scripts/install.sh` when you want the same checksum verification without Homebrew.

## After each `v*` tag

Once `release.yml` has uploaded archives:

```bash
./scripts/gen-homebrew-formula.sh vX.Y.Z > /path/to/homebrew-terradrift/Formula/terradrift.rb
```

Commit in `niravraychura/homebrew-terradrift` (not this repo). Do not point the formula at a tag with no GitHub Release assets yet.
