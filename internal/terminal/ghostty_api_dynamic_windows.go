//go:build windows && !cgo

package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

type ghosttyScrollViewport struct {
	Tag   int32
	_     int32
	Value [2]uint64
}

func loadGhosttyAPI() (*ghosttyAPI, error) {
	ghosttyLoadOnce.Do(func() {
		path, err := findGhosttyDLL()
		if err != nil {
			ghosttyLoadErr = err
			return
		}
		dll, err := syscall.LoadDLL(path)
		if err != nil {
			ghosttyLoadErr = fmt.Errorf("load Ghostty VT DLL %q: %w", path, err)
			return
		}
		find := func(name string) *ghosttyProc {
			p, e := dll.FindProc(name)
			if e != nil {
				if ghosttyLoadErr == nil {
					ghosttyLoadErr = fmt.Errorf("find Ghostty export %s: %w", name, e)
				}
				return nil
			}
			return &ghosttyProc{call: func(args ...uintptr) (uintptr, uintptr, error) {
				return p.Call(args...)
			}}
		}
		a := &ghosttyAPI{
			terminalNew:    find("ghostty_terminal_new"),
			terminalFree:   find("ghostty_terminal_free"),
			terminalSet:    find("ghostty_terminal_set"),
			terminalWrite:  find("ghostty_terminal_vt_write"),
			terminalResize: find("ghostty_terminal_resize"),
			renderNew:      find("ghostty_render_state_new"),
			renderFree:     find("ghostty_render_state_free"),
			renderUpdate:   find("ghostty_render_state_update"),
			renderGet:      find("ghostty_render_state_get"),
			renderClean:    find("ghostty_render_state_clean"),
			rowIteratorNew: find("ghostty_render_state_row_iterator_new"), rowIteratorFree: find("ghostty_render_state_row_iterator_free"),
			rowIteratorNext: find("ghostty_render_state_row_iterator_next"),
			rowGet:          find("ghostty_render_state_row_get"),
			rowCellsNew:     find("ghostty_render_state_row_cells_new"),
			rowCellsFree:    find("ghostty_render_state_row_cells_free"),
			rowCellsNext:    find("ghostty_render_state_row_cells_next"),
			rowCellsGet:     find("ghostty_render_state_row_cells_get"),
		}
		if gridRef := find("ghostty_terminal_grid_ref"); gridRef != nil {
			// The C API takes GhosttyPoint by value. The Win64 ABI passes
			// structs that are not 1/2/4/8 bytes as a pointer, so the
			// address can be passed in the argument slot directly.
			a.terminalGridRef = &ghosttyProc{call: func(args ...uintptr) (uintptr, uintptr, error) {
				if unsafe.Sizeof(uintptr(0)) != 8 {
					return ^uintptr(0), 0, fmt.Errorf("ghostty_terminal_grid_ref requires the Win64 ABI")
				}
				point := ghosttyPoint{
					Tag:   int32(args[1]),
					Value: [2]uint64{uint64(uint16(args[2])) | uint64(uint32(args[3]))<<32},
				}
				r, _, err := gridRef.Call(args[0], uintptr(unsafe.Pointer(&point)), args[4])
				runtime.KeepAlive(&point)
				return r, 0, err
			}}
		}
		if scrollViewport := find("ghostty_terminal_scroll_viewport"); scrollViewport != nil {
			a.terminalScrollViewport = &ghosttyProc{call: func(args ...uintptr) (uintptr, uintptr, error) {
				if unsafe.Sizeof(uintptr(0)) != 8 {
					return ^uintptr(0), 0, fmt.Errorf("ghostty_terminal_scroll_viewport requires the Win64 ABI")
				}
				behavior := ghosttyScrollViewport{
					Tag: int32(args[1]),
				}
				if args[1] == 2 {
					behavior.Value[0] = uint64(int64(int(args[2])))
				} else if args[1] == 3 {
					behavior.Value[0] = uint64(args[3])
				}
				r, _, err := scrollViewport.Call(args[0], uintptr(unsafe.Pointer(&behavior)))
				runtime.KeepAlive(&behavior)
				return r, 0, err
			}}
		}
		a.terminalGet = find("ghostty_terminal_get")
		if ghosttyLoadErr == nil {
			ghostty = a
		} else {
			dll.Release()
		}
	})
	return ghostty, ghosttyLoadErr
}

func findGhosttyDLL() (string, error) {
	if p := os.Getenv("GHOSTTY_VT_DLL"); p != "" {
		return p, nil
	}
	if p := os.Getenv("TERMINAL_LIBGHOSTY_NATIVE_DIR"); p != "" {
		return filepath.Join(p, "ghostty-vt.dll"), nil
	}
	candidates := []string{"build/native-local/ghostty-vt.dll", "../build/native-local/ghostty-vt.dll", "../../build/native-local/ghostty-vt.dll"}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "ghostty-vt.dll"),
			filepath.Join(exeDir, "..", "build", "native-local", "ghostty-vt.dll"),
			filepath.Join(exeDir, "native-local", "ghostty-vt.dll"),
		)
	}
	for _, candidate := range candidates {
		p, err := filepath.Abs(candidate)
		if err == nil {
			if _, statErr := os.Stat(p); statErr == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("ghostty-vt.dll not found; set GHOSTTY_VT_DLL or TERMINAL_LIBGHOSTY_NATIVE_DIR")
}
