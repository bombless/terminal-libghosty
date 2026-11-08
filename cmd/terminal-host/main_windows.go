//go:build windows

// terminal-host embeds a StartSSH-backed terminal child window in the profile GUI.
package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/bombless/terminal-libghosty/internal/ssh"
	"github.com/bombless/terminal-libghosty/internal/terminal"
)

var (
	u                = syscall.NewLazyDLL("user32.dll")
	g                = syscall.NewLazyDLL("gdi32.dll")
	createWindow     = u.NewProc("CreateWindowExW")
	registerClass    = u.NewProc("RegisterClassW")
	defWindowProc    = u.NewProc("DefWindowProcW")
	getMessage       = u.NewProc("GetMessageW")
	translateMessage = u.NewProc("TranslateMessage")
	dispatchMessage  = u.NewProc("DispatchMessageW")
	showWindow       = u.NewProc("ShowWindow")
	updateWindow     = u.NewProc("UpdateWindow")
	postQuit         = u.NewProc("PostQuitMessage")
	beginPaint       = u.NewProc("BeginPaint")
	endPaint         = u.NewProc("EndPaint")
	fillRect         = u.NewProc("FillRect")
	getClientRect    = u.NewProc("GetClientRect")
	moveWindow       = u.NewProc("MoveWindow")
	setTimer         = u.NewProc("SetTimer")
	invalidateRect   = u.NewProc("InvalidateRect")
	setTextColor     = g.NewProc("SetTextColor")
	setBkMode        = g.NewProc("SetBkMode")
	textOut          = g.NewProc("TextOutW")
	createBrush      = g.NewProc("CreateSolidBrush")
	deleteObject     = g.NewProc("DeleteObject")
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
	wmDestroy = 2
	wmSize    = 5
	wmPaint   = 15
	wmChar    = 0x102
	wmKeyDown = 0x100
	wmTimer   = 0x113
	wsChild   = 0x40000000
	wsVisible = 0x10000000
	wsTabStop = 0x10000
)

var sess *terminal.Session
var parent uintptr
var cols, rows uint16 = 80, 24
var processExited bool
var frameReady bool
var frameCols, frameRows uint16
var frameCells []terminal.Cell
var frameText []byte
var frameCursor terminal.Cursor

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
		if sess.Resize(c, rr) == nil {
			cols, rows = c, rr
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
		frameCols, frameRows, frameCells, frameText, frameCursor = cc, rr, cells, text, cursor
		frameReady = true
		invalidateRect.Call(hwnd, 0, 0, 0)
	}
}

func wndproc(hwnd uintptr, m uint32, w, l uintptr) uintptr {
	switch m {
	case wmSize:
		resize(hwnd)
	case wmChar:
		if sess != nil && w <= 0x10ffff {
			_ = sess.Write([]byte(string(rune(w))))
		}
		return 0
	case wmKeyDown:
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
			case 0x1b:
				seq = "\x1b"
			}
			if seq != "" {
				_ = sess.Write([]byte(seq))
				return 0
			}
		}
	case wmTimer:
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
		br, _, _ := createBrush.Call(0x0018120e)
		fillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), br)
		deleteObject.Call(br)
		setBkMode.Call(hdc, 1)
		if sess != nil {
			cc, rr, cells, buf, cur := frameCols, frameRows, frameCells, frameText, frameCursor
			if frameReady {
				var brush uintptr
				var brushColor uint32
				for y := uint16(0); y < rr; y++ {
					for x := uint16(0); x < cc; x++ {
						cell := cells[int(y)*int(cc)+int(x)]
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
						fillRect.Call(hdc, uintptr(unsafe.Pointer(&cr)), brush)
						end := uint64(cell.TextOffset) + uint64(cell.TextLength)
						if cell.TextLength > 0 && end <= uint64(len(buf)) {
							fg := uint32(cell.Foreground.R) | uint32(cell.Foreground.G)<<8 | uint32(cell.Foreground.B)<<16
							setTextColor.Call(hdc, uintptr(fg))
							p, _ := syscall.UTF16FromString(string(buf[cell.TextOffset:uint32(end)]))
							if len(p) > 1 {
								textOut.Call(hdc, uintptr(int(x)*9), uintptr(int(y)*18+2), uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1))
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
					fillRect.Call(hdc, uintptr(unsafe.Pointer(&cr)), caret)
					deleteObject.Call(caret)
				}
			}
		}
		endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmDestroy:
		if sess != nil {
			sess.Close()
			sess = nil
		}
		postQuit.Call(0)
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, uintptr(m), w, l)
	return r
}
func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	log.Print("terminal-host starting")
	var target, key, parentText string
	var port int
	flag.StringVar(&target, "ssh", "", "SSH target user@host[:port]")
	flag.StringVar(&key, "identity", "", "private key path")
	flag.IntVar(&port, "port", 0, "SSH port override")
	flag.StringVar(&parentText, "parent", "", "parent Win32 HWND")
	flag.Parse()
	log.Printf("arguments parsed: target=%q parent=%q port=%d identityConfigured=%t", target, parentText, port, key != "")
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
	wc := WNDCLASS{WndProc: cb, Background: 6, ClassName: name}
	if r, _, e := registerClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && e != syscall.Errno(1410) {
		log.Printf("RegisterClass failed: %v", e)
		sess.Close()
		sess = nil
		return
	}
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
	u.NewProc("SetFocus").Call(hwnd)
	setTimer.Call(hwnd, 1, 75, 0)
	var m MSG
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}
