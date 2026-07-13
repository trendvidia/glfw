// vptr-inject: inject a real left-button drag gesture into a wlroots compositor
// (e.g. headless sway) via zwlr_virtual_pointer_v1, so an integration test can
// exercise the pointer-grab / start_drag path that needs a genuine input serial.
//
// Build:
//   wayland-scanner client-header wlr-virtual-pointer-unstable-v1.xml vp.h
//   wayland-scanner private-code  wlr-virtual-pointer-unstable-v1.xml vp.c
//   cc vptr-inject.c vp.c -lwayland-client -o vptr-inject
//
// Usage: WAYLAND_DISPLAY must be set. Optional argv: <width> <height> (output
// extent, default 1600x1200). It presses left over the centre, drags, releases.
#include <linux/input-event-codes.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <wayland-client.h>
#include "wlr-virtual-pointer-unstable-v1-client-protocol.h"

static struct zwlr_virtual_pointer_manager_v1* manager;
static struct wl_seat* seat;

static void reg_global(void* data, struct wl_registry* reg, uint32_t name,
                       const char* iface, uint32_t version)
{
    if (strcmp(iface, zwlr_virtual_pointer_manager_v1_interface.name) == 0)
        manager = wl_registry_bind(reg, name,
                                   &zwlr_virtual_pointer_manager_v1_interface, 1);
    else if (strcmp(iface, wl_seat_interface.name) == 0)
        seat = wl_registry_bind(reg, name, &wl_seat_interface, 1);
}
static void reg_remove(void* d, struct wl_registry* r, uint32_t n) {}
static const struct wl_registry_listener reg_listener = { reg_global, reg_remove };

static uint32_t now_ms(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint32_t)(ts.tv_sec * 1000 + ts.tv_nsec / 1000000);
}

static void nap(long ms)
{
    struct timespec ts = { ms / 1000, (ms % 1000) * 1000000 };
    nanosleep(&ts, NULL);
}

int main(int argc, char** argv)
{
    uint32_t w = argc > 1 ? (uint32_t)atoi(argv[1]) : 1600;
    uint32_t h = argc > 2 ? (uint32_t)atoi(argv[2]) : 1200;

    struct wl_display* dpy = wl_display_connect(NULL);
    if (!dpy) { fprintf(stderr, "vptr-inject: no display\n"); return 2; }

    struct wl_registry* reg = wl_display_get_registry(dpy);
    wl_registry_add_listener(reg, &reg_listener, NULL);
    wl_display_roundtrip(dpy);

    if (!manager || !seat) {
        fprintf(stderr, "vptr-inject: missing zwlr_virtual_pointer_manager_v1/wl_seat\n");
        return 3;
    }

    struct zwlr_virtual_pointer_v1* vp =
        zwlr_virtual_pointer_manager_v1_create_virtual_pointer(manager, seat);
    wl_display_roundtrip(dpy);

    const uint32_t cx = w / 2, cy = h / 2;

    // Move over the (centre of the) window, then let the toolkit see the enter.
    zwlr_virtual_pointer_v1_motion_absolute(vp, now_ms(), cx, cy, w, h);
    zwlr_virtual_pointer_v1_frame(vp);
    wl_display_flush(dpy);
    nap(400);

    // Press left: creates the implicit grab + input serial; the toolkit's button
    // handler runs and calls StartWaylandDrag during this dispatch.
    zwlr_virtual_pointer_v1_button(vp, now_ms(), BTN_LEFT,
                                   WL_POINTER_BUTTON_STATE_PRESSED);
    zwlr_virtual_pointer_v1_frame(vp);
    wl_display_flush(dpy);
    nap(500);

    // Drag while held: the compositor now routes the drag offer back to the
    // source surface and emits wl_data_offer.source_actions (the v3 event that
    // crashed the incomplete listener).
    for (int i = 0; i < 8; i++) {
        uint32_t x = cx + (i % 2 ? 40 : -40) + i * 8;
        uint32_t y = cy + (i % 3 ? 30 : -20);
        zwlr_virtual_pointer_v1_motion_absolute(vp, now_ms(), x, y, w, h);
        zwlr_virtual_pointer_v1_frame(vp);
        wl_display_flush(dpy);
        nap(150);
    }
    nap(500);

    zwlr_virtual_pointer_v1_button(vp, now_ms(), BTN_LEFT,
                                   WL_POINTER_BUTTON_STATE_RELEASED);
    zwlr_virtual_pointer_v1_frame(vp);
    wl_display_flush(dpy);
    wl_display_roundtrip(dpy);

    zwlr_virtual_pointer_v1_destroy(vp);
    wl_display_disconnect(dpy);
    return 0;
}
