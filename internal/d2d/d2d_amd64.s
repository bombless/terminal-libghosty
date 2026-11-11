//go:build windows && amd64

#include "textflag.h"

// func callDrawBitmap(proc, obj, bitmap, destRect, opacityBits, interpolationMode, srcRect uintptr) uintptr
TEXT ·callDrawBitmap(SB), NOSPLIT, $48-64
	MOVQ proc+0(FP), AX
	MOVQ obj+8(FP), CX
	MOVQ bitmap+16(FP), DX
	MOVQ destRect+24(FP), R8
	MOVQ opacityBits+32(FP), R9
	MOVQ R9, X3
	MOVQ interpolationMode+40(FP), R10
	MOVQ R10, 32(SP)
	MOVQ srcRect+48(FP), R11
	MOVQ R11, 40(SP)
	CALL AX
	MOVQ AX, ret+56(FP)
	RET
