//go:build (linux && wayland) || (freebsd && wayland) || (netbsd && wayland) || (openbsd && wayland)
// +build linux,wayland freebsd,wayland netbsd,wayland openbsd,wayland

package glfw

// WAYLAND reports whether the binary was built with the forced `wayland` build
// tag (a single-backend Wayland build). It is false for the default build even
// though that build now also exposes the Wayland native accessors, because the
// default build additionally compiles the X11 backend and selects between them
// at runtime (GetPlatform). The complementary false definition is in
// not_wayland.go.
const WAYLAND = true
