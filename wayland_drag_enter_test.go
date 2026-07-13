//go:build (linux && !x11 && !wayland) || (linux && wayland)
// +build linux,!x11,!wayland linux,wayland

package glfw

import "testing"

// TestWaylandDragEnterNullWindowGuard is the regression test for glfw#17: a
// data-device drag "enter" carrying a foreign surface (one not tagged as a glfw
// window) resolves to a NULL window, and the drag-enter guard must return false
// without dereferencing it (v1.7.1 crashed with a SIGSEGV at the wl.surface
// field offset, 0x3f8).
//
// It calls the real dragEnterTargetsWindow helper through a C test hook, so
// removing the NULL guard re-crashes this test. No glfwInit or compositor is
// needed — it is pure pointer logic, which is why it can cover a path that no
// headless compositor emits (sway never sends a foreign-surface enter, and
// headless Mutter can't be driven in CI).
func TestWaylandDragEnterNullWindowGuard(t *testing.T) {
	if testDragEnterNullWindowTargets(true) {
		t.Fatal("NULL-window drag enter treated as a target — glfw#17 regression")
	}
	// A URI-less offer is never a target either (covers the offer check too).
	if testDragEnterNullWindowTargets(false) {
		t.Fatal("NULL-window/no-uri enter treated as a target")
	}
}
