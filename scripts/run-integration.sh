#!/usr/bin/env bash
#
# Runs the `glfwintegration` tests against a headless display server, then
# prints total Go statement coverage for the binding.
#
#   GLFW_TEST_BACKEND=wayland scripts/run-integration.sh   # default
#   GLFW_TEST_BACKEND=x11     scripts/run-integration.sh
#
# It brings up its own server (sway's wlroots headless backend, or Xvfb),
# exports the env so GLFW auto-selects that backend, runs the tests, and tears
# the server down on exit. Extra args are forwarded to `go test` (e.g. -run,
# -count=1).
#
# Wayland uses sway (wlroots headless backend) rather than
# `weston --backend=headless` because sway advertises a wl_seat, so the
# per-seat clipboard-selection test has a real owner and round-trips. Weston's
# headless backend is seatless: with the seat-guard fixes in wl_init.c /
# wl_window.c it no longer crashes, but its clipboard is a no-op there.
set -euo pipefail

BACKEND="${GLFW_TEST_BACKEND:-wayland}"
COVERPROFILE="${COVERPROFILE:-cover.out}"
PKG="github.com/trendvidia/glfw"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

pids=()
tmpdirs=()
tmpfiles=()
cleanup() {
  for pid in "${pids[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  for d in "${tmpdirs[@]:-}"; do rm -rf "$d" 2>/dev/null || true; done
  for f in "${tmpfiles[@]:-}"; do rm -f "$f" 2>/dev/null || true; done
}
trap cleanup EXIT

# Compile the wlroots virtual-pointer injector and export GLFW_TEST_VPTR_INJECT
# so TestStartWaylandDragNoCrash (glfw#14) can drive a real drag gesture. Best
# effort: on any missing tool the var stays unset and the test skips itself.
build_vptr_injector() {
  local xml="$SCRIPT_DIR/../testdata/wlr-virtual-pointer-unstable-v1.xml"
  command -v wayland-scanner >/dev/null 2>&1 || { echo "no wayland-scanner; drag test will skip"; return 0; }
  command -v cc >/dev/null 2>&1 || { echo "no cc; drag test will skip"; return 0; }
  [ -f "$xml" ] || { echo "no virtual-pointer protocol xml; drag test will skip"; return 0; }
  local d; d="$(mktemp -d)"; tmpdirs+=("$d")
  wayland-scanner client-header "$xml" "$d/wlr-virtual-pointer-unstable-v1-client-protocol.h" || return 0
  wayland-scanner private-code  "$xml" "$d/wlr-virtual-pointer-unstable-v1-protocol.c" || return 0
  if cc -O2 -I"$d" "$SCRIPT_DIR/vptr-inject.c" \
       "$d/wlr-virtual-pointer-unstable-v1-protocol.c" \
       -lwayland-client -o "$d/vptr-inject" 2>"$d/cc.log"; then
    export GLFW_TEST_VPTR_INJECT="$d/vptr-inject"
    echo "built virtual-pointer injector for the glfw#14 regression test"
  else
    echo "virtual-pointer injector build failed (drag test will skip):" >&2
    cat "$d/cc.log" >&2 || true
  fi
}

# Always use a private XDG_RUNTIME_DIR so the harness's sway socket is the only
# wayland-N socket present (a real desktop session would otherwise leave a
# wayland-0 we might pick up by mistake).
XDG_RUNTIME_DIR="$(mktemp -d)"
tmpdirs+=("$XDG_RUNTIME_DIR")
export XDG_RUNTIME_DIR
chmod 700 "$XDG_RUNTIME_DIR"

case "$BACKEND" in
  wayland)
    unset DISPLAY 2>/dev/null || true
    # 1600x1200 output so the single (sway-tiled fullscreen) window is large and
    # the virtual-pointer injector's centre coordinate lands on it.
    swayconf="$(mktemp)"; tmpfiles+=("$swayconf")
    printf 'output HEADLESS-1 resolution 1600x1200\n' >"$swayconf"
    WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1 \
      sway -c "$swayconf" >/tmp/sway-glfw-it.log 2>&1 &
    pids+=("$!")
    # sway picks the next free wayland-N socket in XDG_RUNTIME_DIR; find it.
    sock=""
    for _ in $(seq 1 100); do
      sock="$(cd "$XDG_RUNTIME_DIR" 2>/dev/null && \
        ls wayland-[0-9]* 2>/dev/null | grep -v '\.lock$' | head -1 || true)"
      [ -n "$sock" ] && [ -S "$XDG_RUNTIME_DIR/$sock" ] && break
      sleep 0.1
    done
    [ -n "$sock" ] && [ -S "$XDG_RUNTIME_DIR/$sock" ] || {
      echo "sway did not create a wayland socket; log:" >&2
      cat /tmp/sway-glfw-it.log >&2 || true
      exit 1
    }
    export WAYLAND_DISPLAY="$sock"
    build_vptr_injector   # enables TestStartWaylandDragNoCrash (glfw#14)
    ;;
  x11)
    unset WAYLAND_DISPLAY 2>/dev/null || true
    export DISPLAY=":99"
    Xvfb "$DISPLAY" -screen 0 1024x768x24 >/tmp/xvfb-glfw-it.log 2>&1 &
    pids+=("$!")
    for _ in $(seq 1 100); do
      xdpyinfo -display "$DISPLAY" >/dev/null 2>&1 && break
      sleep 0.1
    done
    xdpyinfo -display "$DISPLAY" >/dev/null 2>&1 || {
      echo "Xvfb did not come up; log:" >&2
      cat /tmp/xvfb-glfw-it.log >&2 || true
      exit 1
    }
    ;;
  *)
    echo "unknown GLFW_TEST_BACKEND: $BACKEND (want wayland|x11)" >&2
    exit 2
    ;;
esac

echo "== integration tests: backend=$BACKEND =="
go test -tags glfwintegration -covermode=atomic \
  -coverpkg="$PKG" -coverprofile="$COVERPROFILE" -v "$PKG" "$@"

echo "== coverage (Go binding layer) =="
go tool cover -func="$COVERPROFILE" | tail -1
