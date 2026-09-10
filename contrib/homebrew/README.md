# Homebrew

Published tap: [`niravraychura/homebrew-tap`](https://github.com/niravraychura/homebrew-tap).

```bash
brew install niravraychura/tap/terradrift
```

That is `user/tap/formula` (this tap only ships the `terradrift` formula). It installs the GitHub Release archive for your OS/arch and checks `sha256` from `checksums.txt`. Prefer `scripts/install.sh` when you want the same checksum verification without Homebrew.

Already tapped as `niravraychura/terradrift` (old name)?

```bash
brew untap niravraychura/terradrift
brew install niravraychura/tap/terradrift
```

## After each `v*` tag

Once `release.yml` has uploaded archives:

```bash
./scripts/gen-homebrew-formula.sh vX.Y.Z > /path/to/homebrew-tap/Formula/terradrift.rb
```

Commit in `niravraychura/homebrew-tap` (not this repo). Do not point the formula at a tag with no GitHub Release assets yet.
