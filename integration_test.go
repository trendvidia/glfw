//go:build glfwintegration

// Integration tests exercise the GLFW Go binding against a *real* windowing
// backend: an X11 server (e.g. Xvfb) or a Wayland compositor (e.g.
// `weston --backend=headless`). They create windows, pump events and drive the
// native accessors, none of which is possible in a normal headless `go test`,
// so they are gated behind the `glfwintegration` build tag and never run by
// default. Launch them via scripts/run-integration.sh, which brings up a
// headless display server first.
//
// Coverage note: `go test` measures the Go accessor layer (glfw.go, window.go,
// native_*.go, …). The cgo C sources are not instrumented — the point of this
// harness is to prove the Go bindings link and behave against a live server,
// not to cover the vendored C.
package glfw_test

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	glfw "github.com/trendvidia/glfw"
)

// ---------------------------------------------------------------------------
// Main-thread dispatch.
//
// GLFW requires glfwInit and every window/event call to happen on the thread
// that called Init (the "main thread"). The Go test runner invokes Test*
// functions on arbitrary goroutines, so we pin one OS thread in TestMain and
// marshal every GLFW call onto it through do().
// ---------------------------------------------------------------------------

var callQ = make(chan func())

// do runs fn on the pinned main thread and blocks until it returns.
func do(fn func()) {
	done := make(chan struct{})
	callQ <- func() { defer close(done); fn() }
	<-done
}

func TestMain(m *testing.M) {
	runtime.LockOSThread()

	var code int
	go func() {
		do(func() {
			if err := glfw.Init(); err != nil {
				panic("glfw.Init: " + err.Error())
			}
		})
		code = m.Run()
		do(glfw.Terminate)
		close(callQ) // break the dispatch loop below
	}()

	for fn := range callQ {
		fn()
	}
	os.Exit(code)
}

// newHiddenWindow creates a hidden, context-less window and schedules its
// destruction, pumping a few event cycles so the surface is configured.
func newHiddenWindow(t *testing.T) *glfw.Window {
	t.Helper()
	var (
		w   *glfw.Window
		err error
	)
	do(func() {
		glfw.DefaultWindowHints()
		glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
		glfw.WindowHint(glfw.Visible, glfw.False)
		w, err = glfw.CreateWindow(320, 240, t.Name(), nil, nil)
	})
	if err != nil {
		t.Fatalf("CreateWindow: %v", err)
	}
	t.Cleanup(func() { do(w.Destroy) })
	pump(20)
	return w
}

// pump dispatches n rounds of pending events on the main thread.
func pump(n int) {
	do(func() {
		for i := 0; i < n; i++ {
			glfw.PollEvents()
		}
	})
}

// ---------------------------------------------------------------------------
// Backend-agnostic tests.
// ---------------------------------------------------------------------------

func TestVersion(t *testing.T) {
	var major int
	do(func() { major, _, _ = glfw.GetVersion() })
	if major < 3 {
		t.Fatalf("unexpected GLFW major version %d", major)
	}
}

func TestPlatform(t *testing.T) {
	var p glfw.Platform
	do(func() { p = glfw.GetPlatform() })
	switch p {
	case glfw.PlatformX11, glfw.PlatformWayland:
		t.Logf("running on platform %v", p)
	default:
		t.Fatalf("unexpected platform %v", p)
	}
}

func TestWindowLifecycle(t *testing.T) {
	w := newHiddenWindow(t)
	var width, height int
	do(func() {
		w.SetTitle("glfw integration")
		width, height = w.GetSize()
	})
	if width <= 0 || height <= 0 {
		t.Fatalf("bad window size %dx%d", width, height)
	}
}

func TestNativeHandles(t *testing.T) {
	w := newHiddenWindow(t)
	do(func() {
		switch glfw.GetPlatform() {
		case glfw.PlatformWayland:
			if glfw.GetWaylandDisplay() == nil {
				t.Error("nil wl_display")
			}
			if w.GetWaylandWindow() == nil {
				t.Error("nil wl_surface")
			}
		case glfw.PlatformX11:
			if w.GetX11Window() == 0 {
				t.Error("zero X11 window id")
			}
		}
	})
}

// TestClipboardDataRoundtrip exercises the #513 typed-clipboard path against a
// live server: set a custom flavor, let the selection propagate, read it back.
func TestClipboardDataRoundtrip(t *testing.T) {
	// A window gives the server a surface to associate the selection with; GLFW
	// owns the clipboard through its own helper, so we don't reference it here.
	newHiddenWindow(t)
	const mime = "application/x-glfw-integration"
	payload := []byte("integration-payload-\x00\x01\x02\xfe\xff")

	var got []byte
	do(func() {
		glfw.SetClipboardData([]glfw.ClipboardFlavor{{MIMEType: mime, Data: payload}})
	})
	pump(40) // let the compositor/server register the selection
	do(func() { got = glfw.GetClipboardData(mime) })

	if !bytes.Equal(got, payload) {
		t.Fatalf("clipboard roundtrip mismatch:\n got: %q\nwant: %q", got, payload)
	}
}

// ---------------------------------------------------------------------------
// Wayland-only tests.
// ---------------------------------------------------------------------------

// TestStartWaylandDragGuard confirms the drag-out primitive (glfw#10) links and
// executes, and that its input-serial guard refuses a drag when no pointer
// button is held rather than crashing. The positive path (an actual
// send/dnd_finished transfer) needs a held pointer grab + a drop target and is
// out of scope for a headless harness.
func TestStartWaylandDragGuard(t *testing.T) {
	w := newHiddenWindow(t)

	var wayland bool
	do(func() { wayland = glfw.GetPlatform() == glfw.PlatformWayland })
	if !wayland {
		t.Skip("Wayland-only")
	}

	var ok bool
	do(func() {
		ok = w.StartWaylandDrag(
			[]glfw.ClipboardFlavor{{MIMEType: "text/plain;charset=utf-8", Data: []byte("x")}},
			glfw.WaylandDragCopy|glfw.WaylandDragMove,
		)
	})
	if ok {
		t.Fatal("StartWaylandDrag succeeded with no active pointer button serial")
	}
}

// TestStartWaylandDragNoCrash is the regression test for glfw#14: a real drag
// must not crash the process. It needs a genuine pointer-button grab (only a
// real input event carries the serial start_drag requires), which a headless
// server can't otherwise provide, so it drives a wlroots virtual-pointer
// injector (scripts/vptr-inject.c, built by run-integration.sh which exports its
// path as GLFW_TEST_VPTR_INJECT) to press-drag-release over the window.
//
// Under a compositor that binds wl_data_device_manager at version 3 (sway does),
// the drag makes the compositor route an offer back to the source surface and
// emit wl_data_offer.source_actions — the v3 event whose missing listener slot
// crashed v1.7.0. If the drag path regresses, this whole test process aborts
// mid-dispatch and the run fails; a clean run proves the fix holds.
func TestStartWaylandDragNoCrash(t *testing.T) {
	var wayland bool
	do(func() { wayland = glfw.GetPlatform() == glfw.PlatformWayland })
	if !wayland {
		t.Skip("Wayland-only")
	}
	injector := os.Getenv("GLFW_TEST_VPTR_INJECT")
	if injector == "" {
		t.Skip("no GLFW_TEST_VPTR_INJECT injector (needs zwlr_virtual_pointer, e.g. sway)")
	}

	// A visible OpenGL window: the surface must present a real buffer for the
	// compositor to map it and route pointer input (a NoAPI window never does).
	var w *glfw.Window
	var err error
	do(func() {
		glfw.DefaultWindowHints()
		glfw.WindowHint(glfw.Visible, glfw.True)
		w, err = glfw.CreateWindow(800, 600, t.Name(), nil, nil)
	})
	if err != nil {
		t.Fatalf("CreateWindow: %v", err)
	}
	t.Cleanup(func() { do(w.Destroy) })

	var started bool // set on the main thread inside the button callback
	do(func() {
		w.MakeContextCurrent()
		w.SetMouseButtonCallback(func(_ *glfw.Window, _ glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
			if action == glfw.Press {
				// The crash (if regressed) happens on a later dispatch iteration,
				// not here; returning true only means the drag was issued.
				if w.StartWaylandDrag(
					[]glfw.ClipboardFlavor{
						{MIMEType: "text/plain;charset=utf-8", Data: []byte("glfw#14 regression")},
						{MIMEType: "text/uri-list", Data: []byte("file:///tmp/x\r\n")},
					},
					glfw.WaylandDragCopy|glfw.WaylandDragMove,
				) {
					started = true
				}
			}
		})
		w.SwapBuffers() // present so the toplevel maps and gains pointer focus
	})

	// Launch the injector concurrently; it presses, drags and releases (~2.5s).
	inj := exec.Command(injector)
	inj.Env = os.Environ()
	if err := inj.Start(); err != nil {
		t.Skipf("cannot start injector: %v", err)
	}
	defer func() { _ = inj.Process.Kill() }()

	// Pump events on the main thread for long enough to cover the whole gesture
	// and the dispatch iterations that would surface the crash.
	deadline := time.Now().Add(6 * time.Second)
	do(func() {
		for time.Now().Before(deadline) {
			glfw.WaitEventsTimeout(0.05)
			w.SwapBuffers()
		}
	})
	_ = inj.Wait()

	// If the button never reached us, injection didn't work in this environment;
	// skip rather than pass vacuously (the crash path was never exercised).
	if !started {
		t.Skip("virtual-pointer injection did not deliver a button press; drag path not exercised")
	}
	// Reaching here means the drag was started and the process survived the
	// subsequent event dispatch — the glfw#14 crash did not recur.
}
