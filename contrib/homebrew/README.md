# Homebrew

TerraDrift does not publish a tap. After each GitHub Release:

1. Download `checksums.txt` from the release assets.
2. Create a local formula (name it `terradrift.rb`) with `url` pointing at the matching `terradrift_<os>_<arch>.tar.gz` and `sha256` copied from `checksums.txt`.
3. Install with `brew install --formula ./terradrift.rb`.

Prefer `scripts/install.sh` when you want checksum verification without maintaining a formula. That script downloads `checksums.txt` and the archive from the same release tag and refuses to install on mismatch.
