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
