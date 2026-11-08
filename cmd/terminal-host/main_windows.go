//go:build windows

// terminal-host embeds a StartSSH-backed terminal child window in the profile GUI.
package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/bombless/terminal-libghosty/internal/ssh"
	"github.com/bombless/terminal-libghosty/internal/terminal"
)

var (
	u                        = syscall.NewLazyDLL("user32.dll")
	g                        = syscall.NewLazyDLL("gdi32.dll")
	k                        = syscall.NewLazyDLL("kernel32.dll")
	createWindow             = u.NewProc("CreateWindowExW")
	registerClass            = u.NewProc("RegisterClassW")
	defWindowProc            = u.NewProc("DefWindowProcW")
	getMessage               = u.NewProc("GetMessageW")
	translateMessage         = u.NewProc("TranslateMessage")
	dispatchMessage          = u.NewProc("DispatchMessageW")
	showWindow               = u.NewProc("ShowWindow")
	updateWindow             = u.NewProc("UpdateWindow")
	postQuit                 = u.NewProc("PostQuitMessage")
	destroyWindow            = u.NewProc("DestroyWindow")
	isWindow                 = u.NewProc("IsWindow")
	setFocus                 = u.NewProc("SetFocus")
	postMessage              = u.NewProc("PostMessageW")
	getWindowThreadProcessId = u.NewProc("GetWindowThreadProcessId")
	openProcess              = k.NewProc("OpenProcess")
	waitForSingleObject      = k.NewProc("WaitForSingleObject")
	closeHandle              = k.NewProc("CloseHandle")
	beginPaint               = u.NewProc("BeginPaint")
	endPaint                 = u.NewProc("EndPaint")
	fillRect                 = u.NewProc("FillRect")
	getClientRect            = u.NewProc("GetClientRect")
	moveWindow               = u.NewProc("MoveWindow")
	setTimer                 = u.NewProc("SetTimer")
	invalidateRect           = u.NewProc("InvalidateRect")
	setTextColor             = g.NewProc("SetTextColor")
	setBkMode                = g.NewProc("SetBkMode")
	textOut                  = g.NewProc("TextOutW")
	createBrush              = g.NewProc("CreateSolidBrush")
	deleteObject             = g.NewProc("DeleteObject")
	createCompatibleDC       = g.NewProc("CreateCompatibleDC")
	createCompatibleBitmap   = g.NewProc("CreateCompatibleBitmap")
	selectObject             = g.NewProc("SelectObject")
	bitBlt                   = g.NewProc("BitBlt")
	deleteDC                 = g.NewProc("DeleteDC")
	createFont               = g.NewProc("CreateFontW")
)

type POINT struct{ X, Y int32 }
type MSG struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
}
type RECT struct{ Left, Top, Right, Bottom int32 }
type PAINTSTRUCT struct {
	Hdc                uintptr
	Erase              int32
	Paint              RECT
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type WNDCLASS struct {
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
}

const (
	wmDestroy       = 2
	wmSize          = 5
	wmSetFocus      = 7
	wmKillFocus     = 8
	wmClose         = 0x0010
	wmPaint         = 15
	wmEraseBkgnd    = 0x14
	wmMouseActivate = 0x21
	wmGetDlgCode    = 0x0087
	wmKeyDown       = 0x100
	wmChar          = 0x102
	wmTimer         = 0x113
	wmLButtonDown   = 0x201
	dlgcWantAllKeys = 0x0004
	dlgcWantArrows  = 0x0001
	dlgcWantTab     = 0x0002
	dlgcWantChars   = 0x0080
	wsChild         = 0x40000000
	wsVisible       = 0x10000000
	wsTabStop       = 0x10000
	srccopy         = 0x00CC0020
)

var sess *terminal.Session
var parent uintptr
var termFont uintptr
var cols, rows uint16 = 80, 24
var processExited bool
var frameReady bool
var frameCols, frameRows uint16
var frameCells []terminal.Cell
var frameText []byte
var frameCursor terminal.Cursor

type syncFileWriter struct {
	mu sync.Mutex
	f  *os.File
}

func (s *syncFileWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err = s.f.Write(p)
	if err == nil {
		_ = s.f.Sync()
	}
	return
}

func wide(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func resize(hwnd uintptr) {
	if sess == nil {
		return
	}
	var r RECT
	getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	c := uint16(max((r.Right-r.Left)/9, 1))
	rr := uint16(max((r.Bottom-r.Top)/18, 1))
	if c != cols || rr != rows {
		log.Printf("resize requested: (%d,%d) -> (%d,%d)", cols, rows, c, rr)
		if err := sess.Resize(c, rr); err == nil {
			log.Printf("sess.Resize succeeded: cols=%d rows=%d", c, rr)
			cols, rows = c, rr
		} else {
			log.Printf("sess.Resize failed: %v", err)
		}
	}
}

func refreshFrame(hwnd uintptr) {
	if sess == nil {
		return
	}
	cc, rr, cells, text, cursor, err := sess.Snapshot()
	if err != nil {
		log.Printf("terminal snapshot failed: %v", err)
		return
	}
	changed := !frameReady || cc != frameCols || rr != frameRows || cursor != frameCursor || !bytes.Equal(text, frameText)
	if !changed && len(cells) == len(frameCells) {
		for i := range cells {
			if cells[i] != frameCells[i] {
				changed = true
				break
			}
		}
	} else if len(cells) != len(frameCells) {
		changed = true
	}
	if changed {
		log.Printf("refreshFrame: screen changed (cols=%d rows=%d textLen=%d cells=%d cursor=(%d,%d,vis=%d))",
			cc, rr, len(text), len(cells), cursor.X, cursor.Y, cursor.Visible)
		frameCols, frameRows, frameCells, frameText, frameCursor = cc, rr, cells, text, cursor
		frameReady = true
		invalidateRect.Call(hwnd, 0, 0)
		updateWindow.Call(hwnd)
	}
}

func wndproc(hwnd uintptr, m uint32, w, l uintptr) uintptr {
	switch m {
	case wmGetDlgCode:
		return dlgcWantAllKeys | dlgcWantArrows | dlgcWantTab | dlgcWantChars
	case wmClose:
		log.Print("WM_CLOSE received, destroying window")
		destroyWindow.Call(hwnd)
		return 0
	case wmLButtonDown:
		log.Print("WM_LBUTTONDOWN: terminal clicked, restoring keyboard focus")
		setFocus.Call(hwnd)
		return 0
	case wmMouseActivate:
		return 1 // MA_ACTIVATE
	case wmSetFocus:
		log.Print("WM_SETFOCUS: terminal child window gained keyboard focus")
		return 0
	case wmKillFocus:
		log.Print("WM_KILLFOCUS: terminal child window lost keyboard focus")
		return 0
	case wmEraseBkgnd:
		return 1
	case wmSize:
		resize(hwnd)
	case wmChar:
		log.Printf("WM_CHAR: char=0x%04X (%q)", w, rune(w))
		if sess != nil && w <= 0x10ffff {
			buf := []byte(string(rune(w)))
			err := sess.Write(buf)
			if err != nil {
				log.Printf("WM_CHAR: sess.Write failed for %q: %v", rune(w), err)
			} else {
				log.Printf("WM_CHAR: sess.Write succeeded for %q (%d bytes)", rune(w), len(buf))
			}
		}
		return 0
	case wmKeyDown:
		log.Printf("WM_KEYDOWN: vk=0x%02X", w)
		if w == 0x0D {
			log.Print("WM_KEYDOWN: Enter (VK_RETURN) detected")
		}
		if sess != nil {
			seq := ""
			switch w {
			case 0x25:
				seq = "\x1b[D"
			case 0x26:
				seq = "\x1b[A"
			case 0x27:
				seq = "\x1b[C"
			case 0x28:
				seq = "\x1b[B"
			case 0x24:
				seq = "\x1b[H"
			case 0x23:
				seq = "\x1b[F"
			case 0x2d:
				seq = "\x1b[2~"
			case 0x2e:
				seq = "\x1b[3~"
			case 0x21:
				seq = "\x1b[5~"
			case 0x22:
				seq = "\x1b[6~"
			case 0x1b:
				seq = "\x1b"
			}
			if seq != "" {
				log.Printf("WM_KEYDOWN: mapped to escape seq %q", seq)
				err := sess.Write([]byte(seq))
				if err != nil {
					log.Printf("WM_KEYDOWN: sess.Write failed: %v", err)
				}
				return 0
			}
		}
	case wmTimer:
		if parent != 0 {
			r, _, _ := isWindow.Call(parent)
			if r == 0 {
				log.Print("parent window is no longer valid, exiting terminal-host")
				destroyWindow.Call(hwnd)
				return 0
			}
		}
		var parentRect, childRect RECT
		getClientRect.Call(parent, uintptr(unsafe.Pointer(&parentRect)))
		getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&childRect)))
		wantW, wantH := max(parentRect.Right-parentRect.Left, 1), max(parentRect.Bottom-parentRect.Top, 1)
		if childRect.Right-childRect.Left != wantW || childRect.Bottom-childRect.Top != wantH {
			moveWindow.Call(hwnd, 0, 0, uintptr(wantW), uintptr(wantH), 1)
		}
		if sess != nil && !processExited {
			done, code, e := sess.PollExit()
			if e != nil {
				log.Printf("terminal process poll failed: %v", e)
			} else if done {
				processExited = true
				log.Printf("SSH child process exited: code=%d", code)
			}
			refreshFrame(hwnd)
		}
	case wmPaint:
		var ps PAINTSTRUCT
		hdc, _, _ := beginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var r RECT
		getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		w := max(r.Right-r.Left, 1)
		h := max(r.Bottom-r.Top, 1)

		memDC, _, _ := createCompatibleDC.Call(hdc)
		if memDC != 0 {
			memBmp, _, _ := createCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
			if memBmp != 0 {
				oldBmp, _, _ := selectObject.Call(memDC, memBmp)

				br, _, _ := createBrush.Call(0x0018120e)
				fillRect.Call(memDC, uintptr(unsafe.Pointer(&r)), br)
				deleteObject.Call(br)

				setBkMode.Call(memDC, 1)
				var oldFont uintptr
				if termFont != 0 {
					oldFont, _, _ = selectObject.Call(memDC, termFont)
				}

				if sess != nil && frameReady {
					cc, rr, cells, buf, cur := frameCols, frameRows, frameCells, frameText, frameCursor
					var brush uintptr
					var brushColor uint32
					for y := uint16(0); y < rr; y++ {
						for x := uint16(0); x < cc; x++ {
							idx := int(y)*int(cc) + int(x)
							if idx >= len(cells) {
								continue
							}
							cell := cells[idx]
							bg := uint32(cell.Background.R) | uint32(cell.Background.G)<<8 | uint32(cell.Background.B)<<16
							if cell.Selected != 0 {
								bg = 0x00a05028
							}
							if brush == 0 || bg != brushColor {
								if brush != 0 {
									deleteObject.Call(brush)
								}
								brush, _, _ = createBrush.Call(uintptr(bg))
								brushColor = bg
							}
							cr := RECT{Left: int32(x) * 9, Top: int32(y) * 18, Right: int32(x+1) * 9, Bottom: int32(y+1) * 18}
							fillRect.Call(memDC, uintptr(unsafe.Pointer(&cr)), brush)
							end := uint64(cell.TextOffset) + uint64(cell.TextLength)
							if cell.TextLength > 0 && end <= uint64(len(buf)) {
								fg := uint32(cell.Foreground.R) | uint32(cell.Foreground.G)<<8 | uint32(cell.Foreground.B)<<16
								setTextColor.Call(memDC, uintptr(fg))
								p, _ := syscall.UTF16FromString(string(buf[cell.TextOffset:uint32(end)]))
								if len(p) > 1 {
									textOut.Call(memDC, uintptr(int(x)*9), uintptr(int(y)*18+2), uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1))
								}
							}
						}
					}
					if brush != 0 {
						deleteObject.Call(brush)
					}
					if cur.Visible != 0 && cur.X < cc && cur.Y < rr {
						caret, _, _ := createBrush.Call(0x00e8e8e8)
						cr := RECT{Left: int32(cur.X) * 9, Top: int32(cur.Y) * 18, Right: int32(cur.X)*9 + 2, Bottom: int32(cur.Y+1) * 18}
						fillRect.Call(memDC, uintptr(unsafe.Pointer(&cr)), caret)
						deleteObject.Call(caret)
					}
				}

				bitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, srccopy)

				if oldFont != 0 {
					selectObject.Call(memDC, oldFont)
				}
				selectObject.Call(memDC, oldBmp)
				deleteObject.Call(memBmp)
			}
			deleteDC.Call(memDC)
		}
		endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmDestroy:
		log.Print("WM_DESTROY received, cleaning up session and font")
		if sess != nil {
			sess.Close()
			sess = nil
		}
		if termFont != 0 {
			deleteObject.Call(termFont)
			termFont = 0
		}
		postQuit.Call(0)
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, uintptr(m), w, l)
	return r
}

func main() {
	var target, key, parentText, logDir string
	var port int
	flag.StringVar(&target, "ssh", "", "SSH target user@host[:port]")
	flag.StringVar(&key, "identity", "", "private key path")
	flag.IntVar(&port, "port", 0, "SSH port override")
	flag.StringVar(&parentText, "parent", "", "parent Win32 HWND")
	flag.StringVar(&logDir, "log-dir", "", "directory for host logs")
	flag.Parse()

	logOutputs := []io.Writer{os.Stderr}
	if logDir != "" {
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, "host.log")
		if lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			defer lf.Close()
			logOutputs = append(logOutputs, &syncFileWriter{f: lf})
		}
	}
	log.SetOutput(io.MultiWriter(logOutputs...))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	log.Print("terminal-host starting")
	log.Printf("arguments parsed: target=%q parent=%q port=%d identityConfigured=%t logDir=%q", target, parentText, port, key != "", logDir)
	if target == "" || parentText == "" {
		log.Print("startup validation failed: --ssh and --parent are required")
		os.Exit(2)
	}
	base := 10
	if strings.HasPrefix(strings.ToLower(parentText), "0x") {
		base = 16
		parentText = parentText[2:]
	}
	p, e := strconv.ParseUint(parentText, base, 64)
	if e != nil {
		log.Printf("invalid parent HWND: %v", e)
		os.Exit(2)
	}
	parent = uintptr(p)

	// Monitor parent process so terminal-host immediately terminates if parent process dies.
	var parentPid uint32
	getWindowThreadProcessId.Call(parent, uintptr(unsafe.Pointer(&parentPid)))
	if parentPid != 0 {
		log.Printf("detected parent PID: %d", parentPid)
		// SYNCHRONIZE = 0x00100000
		hProc, _, _ := openProcess.Call(0x00100000, 0, uintptr(parentPid))
		if hProc != 0 {
			go func() {
				defer closeHandle.Call(hProc)
				// Wait for parent process to exit
				waitForSingleObject.Call(hProc, 0xFFFFFFFF)
				log.Print("parent process exited, terminating terminal-host")
				time.Sleep(50 * time.Millisecond)
				os.Exit(0)
			}()
		}
	}

	log.Printf("starting SSH ConPTY session: parent=0x%X", parent)
	sess, e = terminal.StartSSH(ssh.Options{Target: target, IdentityFile: key, Port: port}, cols, rows)
	if e != nil {
		log.Printf("StartSSH failed: %v", e)
		os.Exit(1)
	}
	log.Print("SSH ConPTY session created successfully")
	defer func() {
		if sess != nil {
			sess.Close()
		}
	}()

	cb := syscall.NewCallback(wndproc)
	name := wide("TerminalLibghostyEmbedded")
	wc := WNDCLASS{WndProc: cb, Background: 0, ClassName: name}
	if r, _, e := registerClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && e != syscall.Errno(1410) {
		log.Printf("RegisterClass failed: %v", e)
		sess.Close()
		sess = nil
		return
	}
	termFont, _, _ = createFont.Call(18, 9, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, uintptr(1|(3<<4)), uintptr(unsafe.Pointer(wide("Consolas"))))
	var pr RECT
	getClientRect.Call(parent, uintptr(unsafe.Pointer(&pr)))
	hwnd, _, e := createWindow.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(wide("SSH Terminal"))), wsChild|wsVisible|wsTabStop, 0, 0, uintptr(max(pr.Right-pr.Left, 1)), uintptr(max(pr.Bottom-pr.Top, 1)), parent, 0, 0, 0)
	if hwnd == 0 {
		log.Printf("CreateWindow failed: %v", e)
		sess.Close()
		sess = nil
		return
	}
	log.Printf("embedded terminal window created: hwnd=0x%X parent=0x%X", hwnd, parent)
	showWindow.Call(hwnd, 5)
	updateWindow.Call(hwnd)
	setFocus.Call(hwnd)
	setTimer.Call(hwnd, 1, 25, 0)
	var m MSG
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	log.Print("message loop exited, terminal-host ending")
}
