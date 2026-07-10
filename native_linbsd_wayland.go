//go:build (linux && !x11 && !wayland) || (linux && wayland) || (freebsd && wayland) || (netbsd && wayland) || (openbsd && wayland)
// +build linux,!x11,!wayland linux,wayland freebsd,wayland netbsd,wayland openbsd,wayland

package glfw

// The Wayland native accessors are exposed in the default linux build (which
// compiles both the X11 and Wayland C backends, see build.go / c_glfw_lin_*.go)
// alongside the X11 accessors in native_linbsd_x11.go, so a single binary can
// pick the right handle at runtime via GetPlatform(). The forced `wayland` /
// `x11` build tags still narrow to a single backend. The WAYLAND marker const
// lives in wayland_marker.go / not_wayland.go so it keeps meaning "built with
// the wayland tag" rather than "Wayland accessors present".

//#include <stdlib.h>
//#define GLFW_EXPOSE_NATIVE_WAYLAND
//#define GLFW_EXPOSE_NATIVE_EGL
//#define GLFW_INCLUDE_NONE
//#include "glfw/include/GLFW/glfw3.h"
//#include "glfw/include/GLFW/glfw3native.h"
import "C"

import "errors"

func GetWaylandDisplay() *C.struct_wl_display {
	ret := C.glfwGetWaylandDisplay()
	panicError()
	return ret
}

func (m *Monitor) GetWaylandMonitor() *C.struct_wl_output {
	ret := C.glfwGetWaylandMonitor(m.data)
	panicError()
	return ret
}

func (w *Window) GetWaylandWindow() *C.struct_wl_surface {
	ret := C.glfwGetWaylandWindow(w.data)
	panicError()
	return ret
}

// SetWaylandParent establishes an xdg_toplevel.set_parent relationship so the
// compositor keeps w stacked above parent and minimizes them together. Pass a
// nil parent to unset it. Both windows must be mapped toplevels; the call is
// ignored if w has no xdg_toplevel yet. Must be called from the main thread.
//
// This is a trendvidia/glfw extension (not in upstream GLFW): GLFW does not
// otherwise expose the internal xdg_toplevel, so window-hosted dialogs
// (fyne#598) cannot express window parenting on Wayland without it.
func (w *Window) SetWaylandParent(parent *Window) {
	var p *C.GLFWwindow
	if parent != nil {
		p = parent.data
	}
	C.glfwSetWaylandWindowParent(w.data, p)
	panicError()
}

// SetWaylandModal sets or clears native window-modality via the xdg-dialog-v1
// protocol so a supporting compositor blocks input to w's parent (set with
// SetWaylandParent) while w is shown. Pass false to clear it. Compositors
// without the protocol leave this a no-op. Must be called from the main thread.
//
// This is a trendvidia/glfw extension (not in upstream GLFW), the modal twin of
// (*Window).SetWaylandParent, for window-hosted dialogs (fyne#498).
func (w *Window) SetWaylandModal(modal bool) {
	m := C.int(False)
	if modal {
		m = C.int(True)
	}
	C.glfwSetWaylandWindowModal(w.data, m)
	panicError()
}

// ExportWaylandHandle exports w's xdg_toplevel via the xdg-foreign protocol
// (zxdg_exporter_v2) and returns the handle string, for use as an xdg-foreign
// parent reference — for example an XDG portal parent_window of the form
// "wayland:<handle>". It returns an error when the compositor does not advertise
// xdg-foreign. The handle is owned by GLFW and remains valid until the window is
// destroyed; repeated calls return the same handle. Must be called from the main
// thread while w is shown.
//
// This is a trendvidia/glfw extension (not in upstream GLFW), added for parenting
// native portal dialogs on Wayland (fyne#626).
func (w *Window) ExportWaylandHandle() (string, error) {
	h := C.glfwGetWaylandWindowExportHandle(w.data)
	panicError()
	if h == nil {
		return "", errors.New("glfw: xdg-foreign export handle unavailable (compositor lacks zxdg_exporter_v2)")
	}
	return C.GoString(h), nil
}

func GetEGLDisplay() C.EGLDisplay {
	ret := C.glfwGetEGLDisplay()
	panicError()
	return ret
}

func (w *Window) GetEGLContext() C.EGLContext {
	ret := C.glfwGetEGLContext(w.data)
	panicError()
	return ret
}

func (w *Window) GetEGLSurface() C.EGLSurface {
	ret := C.glfwGetEGLSurface(w.data)
	panicError()
	return ret
}
