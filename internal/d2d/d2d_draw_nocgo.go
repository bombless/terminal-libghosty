//go:build windows && !cgo

package d2d

func (rt *RenderTarget) DrawBitmap(bitmap uintptr, destRect RectF, opacity float32, srcRect *RectF) {
}
