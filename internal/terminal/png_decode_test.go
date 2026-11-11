package terminal

import (
	"encoding/base64"
	"testing"
)

func TestDecodePNGBytes(t *testing.T) {
	b64 := "iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAABrUlEQVR4nO2bwXUCMQxETV7qSMpJCuGeVpI7hSTlUAm5ZHlG2X2spJFHPOufF3Y0jAz2isPL5+XSJuaJLYBNGcAWwKYMYAtgUwawBbApA9gC2DyPvuH54/41r1/xOhaGGLCn6K3ro80IM2Cz6OP7/RefvlffJ8KMQ8Rm6F/xe4reojOjNbwJUAOghUuCjIAZcFM8snBJZwTCBMjX4LDixftrF9c13Am4iogufI2/NHiS4EoAtfjuvp4kmA1AxA+JVY9/DWB9+qD7mwygR1/iaAW1AdmiL9Hqs7dAlk9/wahn+u2wyoB0vS8xrAXjzgNOP/rXHN/wOgTVAnsvTB//BWUbVALYAtiUAWwBbMoAtgA2ZQBbAJvdBlzP3cTxdDqU54TTJ2DcZmjAxsaCKgHp28BwTD59C9gNyJYCox61ASOHFyxo9ZkSkG4tcDwi868BbBOc9zcbkK0VrHpcCaC3AuDpMGRAYuh8QGvQIQnI74AbEdFpAE+I1IxQTYkF/WcINSfY8zBzghLPI/WHnRTt6YuYcla4J9sPqNoOswWwKQPYAtiUAWwBbMoAtgA2v9q4hegqZaURAAAAAElFTkSuQmCC"
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	w, h, pix, ok := decodePNGBytes(raw)
	if !ok {
		t.Fatal("decodePNGBytes failed")
	}
	if w != 64 || h != 64 {
		t.Fatalf("unexpected dimensions: %dx%d (expected 64x64)", w, h)
	}
	if len(pix) != 64*64*4 {
		t.Fatalf("unexpected pixel slice length: %d", len(pix))
	}
}
