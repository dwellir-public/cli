#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: $0 <version|tag> [output-path]" >&2
  echo "Example: $0 v0.1.1 ./PKGBUILD" >&2
  exit 1
fi

input_version="$1"
output_path="${2:-PKGBUILD}"

version="${input_version#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: version must be SemVer (e.g. 0.1.1 or v0.1.1). Got: $input_version" >&2
  exit 1
fi

template_path="packaging/aur/PKGBUILD.tmpl"
if [[ ! -f "$template_path" ]]; then
  echo "Error: missing template at $template_path" >&2
  exit 1
fi

checksums_url="https://github.com/dwellir-public/cli/releases/download/v${version}/checksums.txt"
tmp_checksums="$(mktemp)"
trap 'rm -f "$tmp_checksums"' EXIT

curl -fsSL "$checksums_url" -o "$tmp_checksums"

sha_x86_64="$(awk '/dwellir_linux_amd64\.tar\.gz$/ {print $1}' "$tmp_checksums")"
sha_aarch64="$(awk '/dwellir_linux_arm64\.tar\.gz$/ {print $1}' "$tmp_checksums")"

if [[ -z "$sha_x86_64" || -z "$sha_aarch64" ]]; then
  echo "Error: failed to extract required checksums from $checksums_url" >&2
  exit 1
fi

mkdir -p "$(dirname "$output_path")"

sed \
  -e "s/__VERSION__/${version}/g" \
  -e "s/__SHA256_X86_64__/${sha_x86_64}/g" \
  -e "s/__SHA256_AARCH64__/${sha_aarch64}/g" \
  "$template_path" > "$output_path"

echo "Generated $output_path for v$version"
