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

extern _Bool goDecodePng(uintptr_t allocator, uint8_t* data, size_t dataLen, void* out);

static bool tl_decode_png_bridge(void* userdata, const GhosttyAllocator* allocator, const uint8_t* data, size_t data_len, GhosttySysImage* out) {
	(void)userdata;
	return goDecodePng((uintptr_t)allocator, (uint8_t*)data, data_len, (void*)out);
}

static void tl_sys_init(void) {
	ghostty_sys_set(GHOSTTY_SYS_OPT_DECODE_PNG, (const void*)tl_decode_png_bridge);
}

typedef struct {
	uint32_t image_id;
	uint32_t placement_id;
	uint32_t layer;
	int32_t z;
	int32_t col;
	int32_t row;
	uint32_t grid_cols;
	uint32_t grid_rows;
	uint32_t offset_x;
	uint32_t offset_y;
	uint32_t pixel_width;
	uint32_t pixel_height;
	uint32_t src_x;
	uint32_t src_y;
	uint32_t src_width;
	uint32_t src_height;
	uint32_t img_width;
	uint32_t img_height;
	uint32_t format;
	const uint8_t* pixels_ptr;
	size_t pixels_len;
} TlRenderImageRaw;

static int32_t tl_image_iterator_new(uintptr_t out) {
	GhosttyRenderStateImageIterator iterator = NULL;
	GhosttyResult result = ghostty_render_state_image_iterator_new(NULL, &iterator);
	if (out != 0) *(uintptr_t*)out = (uintptr_t)iterator;
	return (int32_t)result;
}

static void tl_image_iterator_free(uintptr_t iterator) {
	ghostty_render_state_image_iterator_free((GhosttyRenderStateImageIterator)iterator);
}

static int32_t tl_render_extract_images(uintptr_t state, uintptr_t terminal, uintptr_t iter, TlRenderImageRaw* out_arr, int32_t max_count) {
	if (state == 0 || terminal == 0 || iter == 0) return 0;
	GhosttyRenderState rstate = (GhosttyRenderState)state;
	GhosttyTerminal term = (GhosttyTerminal)terminal;
	GhosttyRenderStateImageIterator images = (GhosttyRenderStateImageIterator)iter;

	if (ghostty_render_state_get(rstate, GHOSTTY_RENDER_STATE_DATA_IMAGE_ITERATOR, &images) != GHOSTTY_SUCCESS) {
		return 0;
	}

	GhosttyKittyGraphics graphics = NULL;
	if (ghostty_terminal_get(term, GHOSTTY_TERMINAL_DATA_KITTY_GRAPHICS, &graphics) != GHOSTTY_SUCCESS || !graphics) {
		return 0;
	}

	int32_t count = 0;
	while (ghostty_render_state_image_iterator_next(images)) {
		GhosttyImageProtocol protocol;
		if (ghostty_render_state_image_get(images, GHOSTTY_RENDER_STATE_IMAGE_DATA_PROTOCOL, &protocol) != GHOSTTY_SUCCESS) continue;
		if (protocol != GHOSTTY_IMAGE_PROTOCOL_KITTY) continue;

		if (out_arr != NULL && count < max_count) {
			TlRenderImageRaw* item = &out_arr[count];
			memset(item, 0, sizeof(*item));

			GhosttyRenderStateImageData keys[] = {
				GHOSTTY_RENDER_STATE_IMAGE_DATA_IMAGE_ID,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_KITTY_PLACEMENT_ID,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_LAYER,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_Z,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_VIEWPORT_COL,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_VIEWPORT_ROW,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_GRID_COLS,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_GRID_ROWS,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_CELL_OFFSET_X,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_CELL_OFFSET_Y,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_PIXEL_WIDTH,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_PIXEL_HEIGHT,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_SOURCE_X,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_SOURCE_Y,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_SOURCE_WIDTH,
				GHOSTTY_RENDER_STATE_IMAGE_DATA_SOURCE_HEIGHT,
			};
			void* values[] = {
				&item->image_id,
				&item->placement_id,
				&item->layer,
				&item->z,
				&item->col,
				&item->row,
				&item->grid_cols,
				&item->grid_rows,
				&item->offset_x,
				&item->offset_y,
				&item->pixel_width,
				&item->pixel_height,
				&item->src_x,
				&item->src_y,
				&item->src_width,
				&item->src_height,
			};
			size_t key_count = sizeof(keys)/sizeof(keys[0]);
			ghostty_render_state_image_get_multi(images, key_count, keys, values, NULL);

			GhosttyKittyGraphicsImage img = ghostty_kitty_graphics_image(graphics, item->image_id);
			if (img) {
				GhosttyString px = {0};
				GhosttyKittyImageFormat fmt = 0;
				ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_GRAPHICS_IMAGE_DATA_WIDTH, &item->img_width);
				ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_GRAPHICS_IMAGE_DATA_HEIGHT, &item->img_height);
				ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_GRAPHICS_IMAGE_DATA_FORMAT, &fmt);
				ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_GRAPHICS_IMAGE_DATA_PIXELS, &px);
				item->format = (uint32_t)fmt;
				item->pixels_ptr = px.ptr;
				item->pixels_len = px.len;
			}
		}
		count++;
	}
	return count;
}
*/
import "C"
import "unsafe"

//export goDecodePng
func goDecodePng(allocator uintptr, data *C.uint8_t, dataLen C.size_t, outPtr unsafe.Pointer) C.bool {
	if data == nil || dataLen == 0 || outPtr == nil {
		return false
	}
	raw := C.GoBytes(unsafe.Pointer(data), C.int(dataLen))
	w, h, pix, ok := decodePNGBytes(raw)
	if !ok {
		return false
	}
	totalBytes := C.size_t(len(pix))
	mem := C.ghostty_alloc((*C.GhosttyAllocator)(unsafe.Pointer(allocator)), totalBytes)
	if mem == nil {
		return false
	}
	C.memcpy(unsafe.Pointer(mem), unsafe.Pointer(&pix[0]), totalBytes)
	out := (*C.GhosttySysImage)(outPtr)
	out.width = C.uint32_t(w)
	out.height = C.uint32_t(h)
	out.data = (*C.uint8_t)(mem)
	out.data_len = totalBytes
	return true
}

func cExtractImages(render, terminal, iter uintptr) []RenderImage {
	r := C.tl_render_extract_images(C.uintptr_t(render), C.uintptr_t(terminal), C.uintptr_t(iter), nil, 0)
	n := int(r)
	if n <= 0 {
		return nil
	}
	rawItems := make([]C.TlRenderImageRaw, n)
	actual := int(C.tl_render_extract_images(C.uintptr_t(render), C.uintptr_t(terminal), C.uintptr_t(iter), &rawItems[0], C.int32_t(n)))
	if actual <= 0 {
		return nil
	}
	if actual < n {
		rawItems = rawItems[:actual]
	}

	images := make([]RenderImage, 0, len(rawItems))
	for _, item := range rawItems {
		img := RenderImage{
			ImageID:     uint32(item.image_id),
			PlacementID: uint32(item.placement_id),
			Layer:       uint32(item.layer),
			Z:           int32(item.z),
			Col:         int32(item.col),
			Row:         int32(item.row),
			GridCols:    uint32(item.grid_cols),
			GridRows:    uint32(item.grid_rows),
			OffsetX:     uint32(item.offset_x),
			OffsetY:     uint32(item.offset_y),
			Width:       uint32(item.pixel_width),
			Height:      uint32(item.pixel_height),
			SrcX:        uint32(item.src_x),
			SrcY:        uint32(item.src_y),
			SrcWidth:    uint32(item.src_width),
			SrcHeight:   uint32(item.src_height),
			ImgWidth:    uint32(item.img_width),
			ImgHeight:   uint32(item.img_height),
		}
		if item.pixels_ptr != nil && item.pixels_len > 0 && img.ImgWidth > 0 && img.ImgHeight > 0 {
			srcPix := unsafe.Slice((*byte)(unsafe.Pointer(item.pixels_ptr)), int(item.pixels_len))
			if item.format == 0 && len(srcPix) >= int(img.ImgWidth*img.ImgHeight*3) {
				rgba := make([]byte, img.ImgWidth*img.ImgHeight*4)
				pixelCount := int(img.ImgWidth * img.ImgHeight)
				for i := 0; i < pixelCount; i++ {
					rgba[i*4+0] = srcPix[i*3+0]
					rgba[i*4+1] = srcPix[i*3+1]
					rgba[i*4+2] = srcPix[i*3+2]
					rgba[i*4+3] = 255
				}
				img.Pixels = rgba
			} else if len(srcPix) >= int(img.ImgWidth*img.ImgHeight*4) {
				img.Pixels = append([]byte(nil), srcPix[:img.ImgWidth*img.ImgHeight*4]...)
			}
		}
		images = append(images, img)
	}
	return images
}

func loadGhosttyAPI() (*ghosttyAPI, error) {
	ghosttyLoadOnce.Do(func() {
		C.tl_sys_init()
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
			imageIteratorNew: staticGhosttyProc(func(args ...uintptr) uintptr {
				return uintptr(C.tl_image_iterator_new(C.uintptr_t(args[1])))
			}),
			imageIteratorFree: staticGhosttyProc(func(args ...uintptr) uintptr {
				C.tl_image_iterator_free(C.uintptr_t(args[0]))
				return 0
			}),
			extractImages: cExtractImages,
		}
	})
	return ghostty, ghosttyLoadErr
}

func staticGhosttyProc(fn func(args ...uintptr) uintptr) *ghosttyProc {
	return &ghosttyProc{call: func(args ...uintptr) (uintptr, uintptr, error) {
		return fn(args...), 0, nil
	}}
}
