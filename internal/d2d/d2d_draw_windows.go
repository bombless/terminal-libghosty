//go:build windows && cgo

package d2d

/*
#include <stdint.h>

typedef struct {
	float left, top, right, bottom;
} D2DRectF;

typedef void (*DrawBitmapFn)(uintptr_t, uintptr_t, const D2DRectF*, float, uint32_t, const D2DRectF*);

static void d2d_call_draw_bitmap(uintptr_t fn, uintptr_t rt, uintptr_t bmp,
								 float dl, float dt, float dr, float db,
								 float opacity, uint32_t mode,
								 float sl, float st, float sr, float sb, int hasSrc) {
	D2DRectF dst = {dl, dt, dr, db};
	D2DRectF src = {sl, st, sr, sb};
	const D2DRectF* pSrc = hasSrc ? &src : 0;
	((DrawBitmapFn)fn)(rt, bmp, &dst, opacity, mode, pSrc);
}
*/
import "C"
import "unsafe"

func (rt *RenderTarget) DrawBitmap(bitmap uintptr, destRect RectF, opacity float32, srcRect *RectF) {
	if rt == nil || rt.handle == 0 || bitmap == 0 {
		return
	}
	vtbl := *(*uintptr)(unsafe.Pointer(rt.handle))
	proc := *(*uintptr)(unsafe.Pointer(vtbl + 26*unsafe.Sizeof(uintptr(0))))

	hasSrc := C.int(0)
	var sl, st, sr, sb C.float
	if srcRect != nil {
		hasSrc = 1
		sl = C.float(srcRect.Left)
		st = C.float(srcRect.Top)
		sr = C.float(srcRect.Right)
		sb = C.float(srcRect.Bottom)
	}

	C.d2d_call_draw_bitmap(
		C.uintptr_t(proc), C.uintptr_t(rt.handle), C.uintptr_t(bitmap),
		C.float(destRect.Left), C.float(destRect.Top), C.float(destRect.Right), C.float(destRect.Bottom),
		C.float(opacity), 0,
		sl, st, sr, sb, hasSrc,
	)
}
