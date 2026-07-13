//go:build (linux && !x11 && !wayland) || (linux && wayland)
// +build linux,!x11,!wayland linux,wayland

package glfw

//#include <stdlib.h>
//extern int _glfwTestDragEnterTargetsWindowWayland(void* window, void* surface, int offerHasUriList);
import "C"

// testDragEnterNullWindowTargets bridges to the C dragEnterTargetsWindow helper
// for the glfw#17 regression test. It reports whether a data-device drag "enter"
// resolving to a NULL window (a foreign surface — one not tagged as a glfw
// window) is treated as targeting a window; a correct guard makes this false
// without dereferencing the NULL window. cgo is not permitted in _test.go files,
// so the bridge lives here.
func testDragEnterNullWindowTargets(offerHasUriList bool) bool {
	// A non-nil, never-dereferenced surface pointer stands in for the foreign
	// surface; the guard must short-circuit on the NULL window before touching
	// it. malloc(1) is a valid distinct address without a real wl_surface (which
	// would need a compositor).
	surface := C.malloc(1)
	defer C.free(surface)

	u := C.int(0)
	if offerHasUriList {
		u = C.int(1)
	}
	return C._glfwTestDragEnterTargetsWindowWayland(nil, surface, u) != 0
}
