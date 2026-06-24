#!/usr/bin/env bash

# Regenerate the vendored GLFW C sources at the repository root.
#
# This fork (module github.com/trendvidia/glfw) flattens the upstream go-gl
# v3.x/glfw/glfw layout: the C tree lives in ./glfw and the binding package is
# the repository root, so this script no longer takes a target-directory
# argument.
#
# Example:
#   scripts/generate-buildable-source-code.sh                 # use pinned revision
#   scripts/generate-buildable-source-code.sh <glfw_revision> # bump to a revision
#
# WARNING: this regenerates pristine upstream GLFW C. It does NOT carry the
# IME/preedit patches (ported from glfw/glfw#2130) — those must be reapplied
# afterwards (e.g. from a maintained patch series) or they will be lost.

set -euo pipefail

EXEC="$0"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REVISION_FILE="$REPO_ROOT/GLFW_C_REVISION.txt"

usage() {
    echo "usage: $EXEC [glfw_revision]"
    echo "  glfw_revision defaults to the contents of $REVISION_FILE"
    exit "$1"
}

GLFW_REVISION="${1:-}"
if [ -z "$GLFW_REVISION" ]; then
    if [ -r "$REVISION_FILE" ]; then
        GLFW_REVISION="$(cat "$REVISION_FILE")"
    else
        usage 1
    fi
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

generate_dummy_go_files() {
    local glfw_root="$1"
    local deps_import_root="$2"

    find "$glfw_root/deps" "$glfw_root/include" "$glfw_root/src" -type d -print0 |
        while IFS= read -r -d $'\0' d; do
            cat > "$d"/dummy.go <<'EOF'
//go:build required
// +build required

// Package dummy prevents go tooling from stripping the c dependencies.
package dummy
EOF
        done

    {
        cat <<'EOF'
//go:build required
// +build required

// Package dummy prevents go tooling from stripping the c dependencies.
package dummy
EOF
        echo
        echo "import ("
        find "$glfw_root/deps" -mindepth 1 -maxdepth 1 -type d | sort |
            while IFS= read -r dep_dir; do
                dep_name="$(basename "$dep_dir")"
                echo "	_ \"$deps_import_root/$dep_name\""
            done
        echo ")"
    } > "$glfw_root/deps/dummy.go"
}

generate_wayland_protocol_headers() {
    local upstream_root="$1"
    local include_dir="$2"
    local scanner

    scanner="$(command -v wayland-scanner || true)"
    if [ -z "$scanner" ]; then
        echo "$EXEC: wayland-scanner is required but was not found in PATH" >&2
        exit 1
    fi

    local protocol_files=(
        wayland.xml
        viewporter.xml
        xdg-shell.xml
        idle-inhibit-unstable-v1.xml
        pointer-constraints-unstable-v1.xml
        relative-pointer-unstable-v1.xml
        fractional-scale-v1.xml
        xdg-activation-v1.xml
        xdg-decoration-unstable-v1.xml
        # Fork-specific protocols (not shipped by upstream GLFW); overlaid onto
        # the upstream tree from scripts/fork-wayland-protocols before scanning.
        text-input-unstable-v1.xml
        text-input-unstable-v3.xml
    )

    for protocol in "${protocol_files[@]}"; do
        local protocol_path="$upstream_root/deps/wayland/$protocol"
        local protocol_base="${protocol%.xml}"

        "$scanner" client-header "$protocol_path" \
            "$include_dir/${protocol_base}-client-protocol.h"
        "$scanner" private-code "$protocol_path" \
            "$include_dir/${protocol_base}-client-protocol-code.h"
    done
}

WORK_DIR="$TMP_DIR/work"
UPSTREAM_SRC="$WORK_DIR/glfw-src"
AGGREGATE_DIR="$WORK_DIR/glfw-aggregate"

mkdir -p "$UPSTREAM_SRC"
curl -fsSL "https://github.com/glfw/glfw/archive/${GLFW_REVISION}.tar.gz" |
    tar xz --strip-components=1 --directory="$UPSTREAM_SRC"

# Overlay fork-specific Wayland protocols that upstream GLFW does not ship (the
# IME text-input protocols from glfw/glfw#2130). They are vendored into the
# regenerated deps/wayland tree and their client-protocol headers are generated
# alongside upstream's below.
FORK_PROTOCOLS_DIR="$REPO_ROOT/scripts/fork-wayland-protocols"
if [ -d "$FORK_PROTOCOLS_DIR" ]; then
    cp "$FORK_PROTOCOLS_DIR"/*.xml "$UPSTREAM_SRC/deps/wayland/"
fi

mkdir -p "$AGGREGATE_DIR/include"

cp -r "$UPSTREAM_SRC/src" "$AGGREGATE_DIR/src"
cp -r "$UPSTREAM_SRC/include/"* "$AGGREGATE_DIR/include"
cp -r "$UPSTREAM_SRC/deps" "$AGGREGATE_DIR/deps"
cp "$UPSTREAM_SRC/LICENSE.md" "$AGGREGATE_DIR/LICENSE.md"
generate_wayland_protocol_headers "$UPSTREAM_SRC" "$AGGREGATE_DIR/include"

# Keep parity with files historically excluded in go-gl vendoring scripts.
rm -f "$AGGREGATE_DIR"/src/CMakeLists.txt "$AGGREGATE_DIR"/src/*.in

GLFW_DIR="$REPO_ROOT/glfw"
rm -rf "$GLFW_DIR"
mv "$AGGREGATE_DIR" "$GLFW_DIR"

generate_dummy_go_files "$GLFW_DIR" "github.com/trendvidia/glfw/glfw/deps"

# Record the revision the tree was regenerated from.
printf '%s\n' "$GLFW_REVISION" > "$REVISION_FILE"
