//go:build (linux && !x11 && !wayland) || (linux && x11) || (freebsd && !wayland) || (netbsd && !wayland) || (openbsd && !wayland)
// +build linux,!x11,!wayland linux,x11 freebsd,!wayland netbsd,!wayland openbsd,!wayland

package glfw

//#include <stdlib.h>
//#define GLFW_EXPOSE_NATIVE_X11
//#define GLFW_EXPOSE_NATIVE_GLX
//#define GLFW_INCLUDE_NONE
//#include "glfw/include/GLFW/glfw3.h"
//#include "glfw/include/GLFW/glfw3native.h"
import "C"
import "unsafe"

func GetX11Display() *C.Display {
	ret := C.glfwGetX11Display()
	panicError()
	return ret
}

// GetX11Adapter returns the RRCrtc of the monitor.
func (m *Monitor) GetX11Adapter() C.RRCrtc {
	ret := C.glfwGetX11Adapter(m.data)
	panicError()
	return ret
}

// GetX11Monitor returns the RROutput of the monitor.
func (m *Monitor) GetX11Monitor() C.RROutput {
	ret := C.glfwGetX11Monitor(m.data)
	panicError()
	return ret
}

// GetX11Window returns the Window of the window.
func (w *Window) GetX11Window() C.Window {
	ret := C.glfwGetX11Window(w.data)
	panicError()
	return ret
}

// SetX11Parent sets the WM_TRANSIENT_FOR hint so the window manager keeps w
// above parent and minimizes them together. Pass a nil parent to clear it.
// Must be called from the main thread.
//
// This is a trendvidia/glfw extension (not in upstream GLFW), the X11 twin of
// (*Window).SetWaylandParent, for window-hosted dialogs (fyne#598).
func (w *Window) SetX11Parent(parent *Window) {
	var p *C.GLFWwindow
	if parent != nil {
		p = parent.data
	}
	C.glfwSetX11WindowParent(w.data, p)
	panicError()
}

// SetX11Modal sets or clears native window-modality (_NET_WM_STATE_MODAL) so a
// compliant window manager blocks input to w's WM_TRANSIENT_FOR owner while w
// is shown. Call SetX11Parent first to establish that owner; pass false to
// clear the modal state. Must be called from the main thread.
//
// This is a trendvidia/glfw extension (not in upstream GLFW), the modal twin of
// (*Window).SetX11Parent, for window-hosted dialogs (fyne#498).
func (w *Window) SetX11Modal(modal bool) {
	m := C.int(False)
	if modal {
		m = C.int(True)
	}
	C.glfwSetX11WindowModal(w.data, m)
	panicError()
}

// GetGLXContext returns the GLXContext of the window.
func (w *Window) GetGLXContext() C.GLXContext {
	ret := C.glfwGetGLXContext(w.data)
	panicError()
	return ret
}

// GetGLXWindow returns the GLXWindow of the window.
func (w *Window) GetGLXWindow() C.GLXWindow {
	ret := C.glfwGetGLXWindow(w.data)
	panicError()
	return ret
}

// SetX11SelectionString sets the X11 selection string.
func SetX11SelectionString(str string) {
	s := C.CString(str)
	defer C.free(unsafe.Pointer(s))
	C.glfwSetX11SelectionString(s)
}

// GetX11SelectionString gets the X11 selection string.
func GetX11SelectionString() string {
	s := C.glfwGetX11SelectionString()
	return C.GoString(s)
}
