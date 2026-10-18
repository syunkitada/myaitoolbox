#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf -- "$TMP"' EXIT

pass() {
  printf 'ok: %s\n' "$1"
}

expect_make_failure() {
  description="$1"
  expected="$2"
  output="$3"
  shift 3

  if "$@" >"$output" 2>&1; then
    printf 'FAIL: %s unexpectedly succeeded\n' "$description" >&2
    cat "$output" >&2
    return 1
  fi
  if ! grep -Fq -- "$expected" "$output"; then
    printf 'FAIL: %s did not report the expected refusal\n' "$description" >&2
    cat "$output" >&2
    return 1
  fi
}

default_gobin="$(go env GOBIN)"
if [ -z "$default_gobin" ]; then
  gopath="$(go env GOPATH)"
  default_gobin="${gopath%%:*}/bin"
fi
make -C "$ROOT" -n install GOBIN= >"$TMP/empty-gobin.log"
if ! grep -Fq -- "$default_gobin/browser-worker" "$TMP/empty-gobin.log"; then
  printf 'FAIL: empty GOBIN did not resolve to %s\n' "$default_gobin" >&2
  exit 1
fi
pass 'empty GOBIN resolves to the Go installation directory'

mkdir "$TMP/relative-work"
expect_make_failure \
  'relative GOBIN' \
  'GOBIN must be an absolute path: relative-bin' \
  "$TMP/relative-gobin.log" \
  make -C "$TMP/relative-work" -f "$ROOT/Makefile" -o web-build install GOBIN=relative-bin
if [ -e "$TMP/relative-work/relative-bin" ]; then
  printf 'FAIL: relative GOBIN created a path before being rejected\n' >&2
  exit 1
fi
pass 'relative GOBIN is rejected before writing'

mkdir "$TMP/guard-stubs"
for tool in npm npx go; do
  printf '%s\n' '#!/bin/sh' 'printf "unexpected installer command\\n" >&2' 'exit 97' >"$TMP/guard-stubs/$tool"
  chmod +x "$TMP/guard-stubs/$tool"
done
GUARD_PATH="$TMP/guard-stubs:$PATH"
guard_make() {
  PATH="$GUARD_PATH" make "$@"
}

mkdir -p "$TMP/symlink-bin" "$TMP/symlink-target"
printf 'preserve\n' >"$TMP/symlink-target/marker"
ln -s "$TMP/symlink-target" "$TMP/symlink-bin/browser-worker"
expect_make_failure \
  'worker destination symlink' \
  'refusing to install browser worker into non-directory or symlink' \
  "$TMP/symlink.log" \
  guard_make -C "$ROOT" -o web-build install "GOBIN=$TMP/symlink-bin"
if [ ! -L "$TMP/symlink-bin/browser-worker" ] || [ "$(cat "$TMP/symlink-target/marker")" != 'preserve' ]; then
  printf 'FAIL: worker destination symlink or its target was modified\n' >&2
  exit 1
fi
pass 'worker destination symlink and target are preserved'

mkdir -p "$TMP/unrelated-bin/browser-worker"
printf 'user data\n' >"$TMP/unrelated-bin/browser-worker/keep.txt"
expect_make_failure \
  'unrelated worker directory' \
  'refusing to overwrite an unrelated directory' \
  "$TMP/unrelated.log" \
  guard_make -C "$ROOT" -o web-build install "GOBIN=$TMP/unrelated-bin"
if [ "$(cat "$TMP/unrelated-bin/browser-worker/keep.txt")" != 'user data' ]; then
  printf 'FAIL: unrelated worker directory was modified\n' >&2
  exit 1
fi
pass 'unrelated worker directory is preserved'

mkdir -p "$TMP/nested-bin/browser-worker/src"
printf '{"name":"mybox-browser-worker"}\n' >"$TMP/nested-bin/browser-worker/package.json"
ln -s "$TMP/symlink-target/marker" "$TMP/nested-bin/browser-worker/src/linked-file"
expect_make_failure \
  'nested worker symlink' \
  'refusing to update browser worker paths containing symlinks' \
  "$TMP/nested-symlink.log" \
  guard_make -C "$ROOT" -o web-build install "GOBIN=$TMP/nested-bin"
if [ ! -L "$TMP/nested-bin/browser-worker/src/linked-file" ]; then
  printf 'FAIL: nested worker symlink was modified\n' >&2
  exit 1
fi
pass 'symlinks inside an existing worker are rejected and preserved'

printf 'installer Makefile tests passed\n'
