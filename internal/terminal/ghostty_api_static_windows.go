//go:build windows && cgo

package terminal

/*
#cgo CFLAGS: -DGHOSTTY_STATIC -I${SRCDIR}/../../build/native-static/include
#cgo LDFLAGS: -L${SRCDIR}/../../build/native-static -lghostty-vt-static -static -static-libgcc -lntdll -lkernel32 -luserenv -lws2_32 -lbcrypt -lole32 -ladvapi32

#include <stdint.h>
#include <stddef.h>
#include <string.h>
#include "ghostty/vt.h"

static int32_t tl_terminal_new(uintptr_t out, uint16_t cols, uint16_t rows) {
	GhosttyTerminal terminal = NULL;
	GhosttyResult result = ghostty_terminal_new(NULL, &terminal, cols, rows);
	if (out != 0) *(uintptr_t*)out = (uintptr_t)terminal;
	return (int32_t)result;
}

static void tl_terminal_free(uintptr_t terminal) {
	ghostty_terminal_free((GhosttyTerminal)terminal);
}

static int32_t tl_terminal_set(uintptr_t terminal, int32_t option, uintptr_t value) {
	return (int32_t)ghostty_terminal_set((GhosttyTerminal)terminal, (GhosttyTerminalOption)option, (const void*)value);
}

static void tl_terminal_vt_write(uintptr_t terminal, uintptr_t data, size_t len) {
	ghostty_terminal_vt_write((GhosttyTerminal)terminal, (const uint8_t*)data, len);
}

static int32_t tl_terminal_resize(uintptr_t terminal, uint16_t cols, uint16_t rows, uint32_t cell_width, uint32_t cell_height) {
	return (int32_t)ghostty_terminal_resize((GhosttyTerminal)terminal, cols, rows, cell_width, cell_height);
}

static int32_t tl_terminal_scroll_viewport(uintptr_t terminal, int32_t tag, intptr_t delta, size_t row) {
	GhosttyTerminalScrollViewport behavior;
	memset(&behavior, 0, sizeof(behavior));
	behavior.tag = (GhosttyTerminalScrollViewportTag)tag;
	if (tag == 2) {
		behavior.value.delta = delta;
	} else if (tag == 3) {
		behavior.value.row = row;
	}
	return (int32_t)ghostty_terminal_scroll_viewport((GhosttyTerminal)terminal, behavior);
}

static int32_t tl_terminal_get(uintptr_t terminal, int32_t data, uintptr_t out) {
	return (int32_t)ghostty_terminal_get((GhosttyTerminal)terminal, (GhosttyTerminalData)data, (void*)out);
}

static int32_t tl_terminal_grid_ref(uintptr_t terminal, int32_t tag, uint16_t x, uint32_t y, uintptr_t out) {
	GhosttyPoint point = {0};
	point.tag = (GhosttyPointTag)tag;
	point.value.coordinate.x = x;
	point.value.coordinate.y = y;
	return (int32_t)ghostty_terminal_grid_ref((GhosttyTerminal)terminal, point, (GhosttyGridRef*)out);
}

static int32_t tl_render_state_new(uintptr_t out) {
	GhosttyRenderState state = NULL;
	GhosttyResult result = ghostty_render_state_new(NULL, &state);
	if (out != 0) *(uintptr_t*)out = (uintptr_t)state;
	return (int32_t)result;
}

static void tl_render_state_free(uintptr_t state) {
	ghostty_render_state_free((GhosttyRenderState)state);
}

static int32_t tl_render_state_update(uintptr_t state, uintptr_t terminal) {
	return (int32_t)ghostty_render_state_update((GhosttyRenderState)state, (GhosttyTerminal)terminal);
}

static int32_t tl_render_state_get(uintptr_t state, int32_t data, uintptr_t out) {
	return (int32_t)ghostty_render_state_get((GhosttyRenderState)state, (GhosttyRenderStateData)data, (void*)out);
}

static int32_t tl_render_state_clean(uintptr_t state) {
	return (int32_t)ghostty_render_state_clean((GhosttyRenderState)state);
}

static int32_t tl_row_iterator_new(uintptr_t out) {
	GhosttyRenderStateRowIterator iterator = NULL;
	GhosttyResult result = ghostty_render_state_row_iterator_new(NULL, &iterator);
	if (out != 0) *(uintptr_t*)out = (uintptr_t)iterator;
	return (int32_t)result;
}

static void tl_row_iterator_free(uintptr_t iterator) {
	ghostty_render_state_row_iterator_free((GhosttyRenderStateRowIterator)iterator);
}

static int32_t tl_row_iterator_next(uintptr_t iterator) {
	return ghostty_render_state_row_iterator_next((GhosttyRenderStateRowIterator)iterator) ? 1 : 0;
}

static int32_t tl_row_get(uintptr_t iterator, int32_t data, uintptr_t out) {
	return (int32_t)ghostty_render_state_row_get((GhosttyRenderStateRowIterator)iterator, (GhosttyRenderStateRowData)data, (void*)out);
}

static int32_t tl_row_cells_new(uintptr_t out) {
	GhosttyRenderStateRowCells cells = NULL;
	GhosttyResult result = ghostty_render_state_row_cells_new(NULL, &cells);
	if (out != 0) *(uintptr_t*)out = (uintptr_t)cells;
	return (int32_t)result;
}

static void tl_row_cells_free(uintptr_t cells) {
	ghostty_render_state_row_cells_free((GhosttyRenderStateRowCells)cells);
}

static int32_t tl_row_cells_next(uintptr_t cells) {
	return ghostty_render_state_row_cells_next((GhosttyRenderStateRowCells)cells) ? 1 : 0;
}

static int32_t tl_row_cells_get(uintptr_t cells, int32_t data, uintptr_t out) {
	return (int32_t)ghostty_render_state_row_cells_get((GhosttyRenderStateRowCells)cells, (GhosttyRenderStateRowCellsData)data, (void*)out);
}
*/
import "C"

func loadGhosttyAPI() (*ghosttyAPI, error) {
	ghosttyLoadOnce.Do(func() {
		ghostty = &ghosttyAPI{
			terminalNew: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_terminal_new(C.uintptr_t(args[1]), C.uint16_t(args[2]), C.uint16_t(args[3])))
			}),
			terminalFree: staticGhosttyProc(func(args ...uintptr) uintptr {
				C.tl_terminal_free(C.uintptr_t(args[0]))
				return 0
			}),
			terminalSet: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_terminal_set(C.uintptr_t(args[0]), C.int32_t(args[1]), C.uintptr_t(args[2])))
			}),
			terminalWrite: staticGhosttyProc(func(args ...uintptr) uintptr {
				C.tl_terminal_vt_write(C.uintptr_t(args[0]), C.uintptr_t(args[1]), C.size_t(args[2]))
				return 0
			}),
			terminalResize: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_terminal_resize(C.uintptr_t(args[0]), C.uint16_t(args[1]), C.uint16_t(args[2]), C.uint32_t(args[3]), C.uint32_t(args[4])))
			}),
			terminalScrollViewport: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_terminal_scroll_viewport(C.uintptr_t(args[0]), C.int32_t(args[1]), C.intptr_t(args[2]), C.size_t(args[3])))
			}),
			terminalGet: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_terminal_get(C.uintptr_t(args[0]), C.int32_t(args[1]), C.uintptr_t(args[2])))
			}),
			terminalGridRef: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_terminal_grid_ref(C.uintptr_t(args[0]), C.int32_t(args[1]), C.uint16_t(args[2]), C.uint32_t(args[3]), C.uintptr_t(args[4])))
			}),
			renderNew: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_render_state_new(C.uintptr_t(args[1])))
			}),
			renderFree: staticGhosttyProc(func(args ...uintptr) uintptr {
				C.tl_render_state_free(C.uintptr_t(args[0]))
				return 0
			}),
			renderUpdate: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_render_state_update(C.uintptr_t(args[0]), C.uintptr_t(args[1])))
			}),
			renderGet: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_render_state_get(C.uintptr_t(args[0]), C.int32_t(args[1]), C.uintptr_t(args[2])))
			}),
			renderClean: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_render_state_clean(C.uintptr_t(args[0])))
			}),
			rowIteratorNew: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_row_iterator_new(C.uintptr_t(args[1])))
			}),
			rowIteratorFree: staticGhosttyProc(func(args ...uintptr) uintptr {
				C.tl_row_iterator_free(C.uintptr_t(args[0]))
				return 0
			}),
			rowIteratorNext: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_row_iterator_next(C.uintptr_t(args[0])))
			}),
			rowGet: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_row_get(C.uintptr_t(args[0]), C.int32_t(args[1]), C.uintptr_t(args[2])))
			}),
			rowCellsNew: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_row_cells_new(C.uintptr_t(args[1])))
			}),
			rowCellsFree: staticGhosttyProc(func(args ...uintptr) uintptr {
				C.tl_row_cells_free(C.uintptr_t(args[0]))
				return 0
			}),
			rowCellsNext: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_row_cells_next(C.uintptr_t(args[0])))
			}),
			rowCellsGet: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_row_cells_get(C.uintptr_t(args[0]), C.int32_t(args[1]), C.uintptr_t(args[2])))
			}),
		}
	})
	return ghostty, ghosttyLoadErr
}

func staticGhosttyProc(fn func(args ...uintptr) uintptr) *ghosttyProc {
	return &ghosttyProc{call: func(args ...uintptr) (uintptr, uintptr, error) {
		return fn(args...), 0, nil
	}}
}
