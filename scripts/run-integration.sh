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

pids=()
tmpdirs=()
tmpfiles=()
cleanup() {
  for pid in "${pids[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  for d in "${tmpdirs[@]:-}"; do rm -rf "$d" 2>/dev/null || true; done
  for f in "${tmpfiles[@]:-}"; do rm -f "$f" 2>/dev/null || true; done
}
trap cleanup EXIT

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
    swayconf="$(mktemp)"; tmpfiles+=("$swayconf")
    printf 'output HEADLESS-1 resolution 800x600\n' >"$swayconf"
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
