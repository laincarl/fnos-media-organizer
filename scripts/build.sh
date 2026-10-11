#!/usr/bin/env bash
set -euo pipefail
project=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$project"
version=$(sed -n 's/^version=//p' packaging/fnos/manifest | tr -d '\r')
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid manifest version' >&2; exit 1; }
mkdir -p dist .tools
if [[ -z "${FNPACK:-}" ]]; then
  FNPACK="$project/.tools/fnpack-1.2.3-linux-amd64"
  if [[ ! -f "$FNPACK" ]]; then
    curl --fail --location --retry 3 https://static2.fnnas.com/fnpack/fnpack-1.2.3-linux-amd64 -o "$FNPACK"
  fi
fi
FNPACK=$(realpath "$FNPACK")
printf '%s  %s\n' '54b97fa7b70968c4d05c79840f5daeff508957d0bb2062fdb0376d00d9615c93' "$FNPACK" | sha256sum --check
chmod 755 "$FNPACK"
stage=$(mktemp -d)
cleanup() {
  if [[ -f "$stage/data/app.pid" ]]; then "$stage/package/cmd/main" stop || true; fi
  rm -rf -- "$stage"
}
trap cleanup EXIT
mkdir -p "$stage/package/app/bin"
cp -R packaging/fnos/. "$stage/package/"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o "$stage/package/app/bin/media-organizer" ./cmd/fnos-media-organizer
find "$stage/package" -type d -exec chmod 755 {} +
find "$stage/package" -type f -exec chmod 644 {} +
chmod 755 "$stage/package/app/bin/media-organizer" "$stage/package"/cmd/*
(cd "$stage/package" && "$FNPACK" build)
export TRIM_APPDEST="$stage/package/app" TRIM_PKGVAR="$stage/data"
mkdir -p "$TRIM_PKGVAR/media"
export TRIM_DATA_ACCESSIBLE_PATHS="$TRIM_PKGVAR/media"
"$stage/package/cmd/install_callback"
"$stage/package/cmd/main" start
"$stage/package/cmd/main" status
curl --fail --silent --unix-socket "$TRIM_APPDEST/app.sock" -H 'X-Trim-Userid: 1000' -H 'X-Trim-Username: admin' -H 'X-Trim-Isadmin: true' http://localhost/app/media-organizer/ > "$stage/index.html"
grep -q '飞牛影视整理器' "$stage/index.html"
curl --fail --silent --unix-socket "$TRIM_APPDEST/app.sock" -H 'X-Trim-Userid: 1000' -H 'X-Trim-Username: admin' -H 'X-Trim-Isadmin: true' http://localhost/app/media-organizer/app-icon.png > "$stage/icon.png"
cmp web/app-icon.png "$stage/icon.png"
"$stage/package/cmd/main" stop
if "$stage/package/cmd/main" status; then echo 'Service failed to stop' >&2; exit 1; fi
cp "$stage/package/media-organizer.fpk" "dist/fnos-media-organizer_${version}_x86.fpk"
(cd dist && sha256sum "fnos-media-organizer_${version}_x86.fpk" > "fnos-media-organizer_${version}_x86.fpk.sha256")
echo "Built and verified version $version"
