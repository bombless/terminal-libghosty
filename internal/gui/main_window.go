//go:build windows

package gui

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func init() {
	runtime.LockOSThread()
}

const (
	mainWindowClassName  = "TerminalLibghostyMainWindow"
	panelWindowClassName = "TerminalLibghostyPanel"
	tabStripClassName    = "TerminalLibghostyTabStrip"

	// Control IDs
	idListProfiles   = 1001
	idEditName       = 1002
	idEditTarget     = 1003
	idEditPort       = 1004
	idEditIdentity   = 1005
	idBtnSave        = 1006
	idBtnDelete      = 1007
	idBtnConnect     = 1008
	idHeaderLabel    = 1011
	idSubheaderLabel = 1012
	idBtnToggleList  = 1013
	idBtnLocalShell  = 1014
	idBtnNewSSH      = 1015

	lbsNotify     = 0x0001
	wsVScroll     = 0x00200000
	esAutohscroll = 0x0080
	bsOwnerdraw   = 0x000B

	lbAddstring    = 0x0180
	lbResetcontent = 0x0184
	lbGetcursel    = 0x0188
	lbSetcursel    = 0x0186
	lbnSelchange   = 1
	lbnDblclk      = 2

	dtCenter     = 0x0001
	dtVcenter    = 0x0004
	dtSingleline = 0x0020

	idcArrow = 32512
	vkF9     = 0x78
)

var (
	procLoadCursorW      = u32.NewProc("LoadCursorW")
	procIsDialogMessageW = u32.NewProc("IsDialogMessageW")
	targetRegex          = regexp.MustCompile(`^[A-Za-z0-9._-]+@(?:\[[0-9a-fA-F:]+\]|[A-Za-z0-9.-]+)(?::[0-9]{1,5})?$`)
)

type MainWindow struct {
	hwnd       uintptr
	bgBrush    uintptr
	panelBrush uintptr
	editBrush  uintptr
	btnBrush   uintptr
	headerFont uintptr
	subFont    uintptr
	normalFont uintptr
	btnFont    uintptr

	// Child controls
	hHeader        uintptr
	hSub           uintptr
	hList          uintptr
	hRightPanel    uintptr
	hTabStrip      uintptr
	hConfigPanel   uintptr
	hBtnToggleList uintptr
	hBtnNewSSH     uintptr
	hBtnLocalShell uintptr
	listCollapsed  bool

	// Session tabs; tabs[0] is the always-present SSH config form.
	tabs      []*sessionTab
	activeTab int
	nextTabID uint32

	// Profile fields
	hLblName     uintptr
	hEditName    uintptr
	hLblTarget   uintptr
	hEditTarget  uintptr
	hLblPort     uintptr
	hEditPort    uintptr
	hLblIdentity uintptr
	hEditIdent   uintptr
	hBtnSave     uintptr
	hBtnDelete   uintptr
	hBtnConnect  uintptr

	profiles []Profile
}

func getWindowText(hwnd uintptr) string {
	lenVal, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if lenVal == 0 {
		return ""
	}
	buf := make([]uint16, lenVal+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(lenVal+1))
	return syscall.UTF16ToString(buf)
}

func setWindowText(hwnd uintptr, text string) {
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func mainWndProc(hwnd uintptr, m uint32, w, l uintptr) uintptr {
	ptr, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlpUserData))
	mw := (*MainWindow)(unsafe.Pointer(ptr))

	switch m {
	case wmCtlColorStatic:
		hdc := w
		ctl := l
		procSetBkMode.Call(hdc, 1) // TRANSPARENT
		if mw != nil {
			if ctl == mw.hHeader {
				procSetTextColor.Call(hdc, 0x00FFFFFF)
				return mw.bgBrush
			}
			if ctl == mw.hSub {
				procSetTextColor.Call(hdc, 0x00C0C0C0) // Silver
				return mw.bgBrush
			}
			procSetTextColor.Call(hdc, 0x00C0C0C0)
			return mw.panelBrush
		}
		return 0

	case wmCtlColorEdit:
		hdc := w
		procSetTextColor.Call(hdc, 0x00FFFFFF)
		procSetBkColor.Call(hdc, 0x001A130F)
		if mw != nil {
			return mw.editBrush
		}
		return 0

	case wmCtlColorListbox:
		hdc := w
		procSetTextColor.Call(hdc, 0x00FFFFFF)
		procSetBkColor.Call(hdc, 0x002C211B)
		if mw != nil {
			return mw.panelBrush
		}
		return 0

	case wmDrawItem:
		dis := (*drawItemStruct)(unsafe.Pointer(l))
		if dis.CtlType == 4 && mw != nil { // ODT_BUTTON
			btnText := ""
			fillBrush := mw.btnBrush
			switch dis.CtlID {
			case idBtnSave:
				btnText = "SAVE CONNECTION"
			case idBtnConnect:
				btnText = "CONNECT  →"
			case idBtnDelete:
				btnText = "DELETE"
			case idBtnNewSSH:
				btnText = "NEW SSH  +"
			case idBtnLocalShell:
				btnText = "NEW PWSH  +"
			case idBtnToggleList:
				if mw.listCollapsed {
					btnText = "SHOW LIST  ▶"
				} else {
					btnText = "◀  HIDE LIST"
				}
				fillBrush = mw.panelBrush
			}

			// Fill button background
			procFillRect.Call(dis.Hdc, uintptr(unsafe.Pointer(&dis.RcItem)), fillBrush)
			procSetBkMode.Call(dis.Hdc, 1) // TRANSPARENT
			procSetTextColor.Call(dis.Hdc, 0x00FFFFFF)
			if mw.btnFont != 0 {
				procSelectObject.Call(dis.Hdc, mw.btnFont)
			}
			if btnText != "" {
				wText, _ := syscall.UTF16FromString(btnText)
				procDrawTextW.Call(
					dis.Hdc,
					uintptr(unsafe.Pointer(&wText[0])),
					uintptr(len(wText)-1),
					uintptr(unsafe.Pointer(&dis.RcItem)),
					dtCenter|dtVcenter|dtSingleline,
				)
			}
			return 1
		}

	case wmCommand:
		id := uint16(w & 0xFFFF)
		code := uint16((w >> 16) & 0xFFFF)
		if mw != nil {
			mw.handleCommand(id, code)
		}
		return 0

	case wmSize:
		if mw != nil {
			mw.layout()
		}
		return 0

	case wmAppSessionExit:
		if mw != nil {
			mw.closeTabByID(uint32(w))
		}
		return 0

	case wmSetFocus:
		return 0

	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		if mw != nil {
			mw.cleanup()
		}
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(m), w, l)
	return r
}

func panelWndProc(hwnd uintptr, m uint32, w, l uintptr) uintptr {
	ptr, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlpUserData))
	mw := (*MainWindow)(unsafe.Pointer(ptr))

	switch m {
	case wmEraseBkgnd:
		if mw != nil {
			hdc := w
			var r rect
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), mw.panelBrush)
			return 1
		}
	case wmCtlColorStatic:
		hdc := w
		procSetBkMode.Call(hdc, 1)
		procSetTextColor.Call(hdc, 0x00C0C0C0)
		if mw != nil {
			return mw.panelBrush
		}
	case wmCtlColorEdit:
		hdc := w
		procSetTextColor.Call(hdc, 0x00FFFFFF)
		procSetBkColor.Call(hdc, 0x001A130F)
		if mw != nil {
			return mw.editBrush
		}
	case wmDrawItem:
		if mw != nil {
			return mainWndProc(mw.hwnd, m, w, l)
		}
	case wmCommand:
		id := uint16(w & 0xFFFF)
		code := uint16((w >> 16) & 0xFFFF)
		if mw != nil {
			mw.handleCommand(id, code)
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(m), w, l)
	return r
}

func NewMainWindow() (*MainWindow, error) {
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)

	// Register main window class
	cb := syscall.NewCallback(mainWndProc)
	clsName := utf16Ptr(mainWindowClassName)
	bgBrush, _, _ := procCreateSolidBrush.Call(0x001E1612) // #12161E
	wc := wndClass{
		WndProc:    cb,
		Background: bgBrush,
		ClassName:  clsName,
		Cursor:     cursor,
	}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))

	// Register panel window class
	panelCb := syscall.NewCallback(panelWndProc)
	panelClsName := utf16Ptr(panelWindowClassName)
	panelBrush, _, _ := procCreateSolidBrush.Call(0x002C211B) // #1B212C
	pwc := wndClass{
		WndProc:    panelCb,
		Background: panelBrush,
		ClassName:  panelClsName,
		Cursor:     cursor,
	}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&pwc)))

	// Register tab strip window class
	tabCb := syscall.NewCallback(tabStripWndProc)
	tabClsName := utf16Ptr(tabStripClassName)
	twc := wndClass{
		WndProc:   tabCb,
		ClassName: tabClsName,
		Cursor:    cursor,
	}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&twc)))

	editBrush, _, _ := procCreateSolidBrush.Call(0x001A130F) // #0F131A
	btnBrush, _, _ := procCreateSolidBrush.Call(0x00D27030)  // #3070D2

	// Fonts
	headerFont, _, _ := procCreateFontW.Call(32, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))
	subFont, _, _ := procCreateFontW.Call(18, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))
	normalFont, _, _ := procCreateFontW.Call(19, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))
	btnFont, _, _ := procCreateFontW.Call(17, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))

	mw := &MainWindow{
		bgBrush:    bgBrush,
		panelBrush: panelBrush,
		editBrush:  editBrush,
		btnBrush:   btnBrush,
		headerFont: headerFont,
		subFont:    subFont,
		normalFont: normalFont,
		btnFont:    btnFont,
	}

	title := utf16Ptr("Terminal Libghosty - SSH Manager")
	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow|wsClipChildren,
		0x80000000, 0x80000000, // CW_USEDEFAULT
		1000, 680,
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW failed: %v", callErr)
	}
	mw.hwnd = hwnd
	procSetWindowLongPtrW.Call(hwnd, uintptr(gwlpUserData), uintptr(unsafe.Pointer(mw)))

	mw.createControls()
	mw.loadAndRefreshProfiles()
	mw.layout()

	return mw, nil
}

func (mw *MainWindow) createControls() {
	// Header & Subheader
	mw.hHeader, _, _ = procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(utf16Ptr("STATIC"))), uintptr(unsafe.Pointer(utf16Ptr("TERMINAL LIBGHOSTY"))),
		wsChild|wsVisible, 28, 20, 500, 40, mw.hwnd, idHeaderLabel, 0, 0,
	)
	procSendMessageW.Call(mw.hHeader, 0x0030, mw.headerFont, 1)

	mw.hSub, _, _ = procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(utf16Ptr("STATIC"))), uintptr(unsafe.Pointer(utf16Ptr("SSH CONNECTION MANAGER"))),
		wsChild|wsVisible, 31, 65, 500, 25, mw.hwnd, idSubheaderLabel, 0, 0,
	)
	procSendMessageW.Call(mw.hSub, 0x0030, mw.subFont, 1)

	// Top bar buttons (positioned by layout)
	createTopBtn := func(text string, id uintptr) uintptr {
		b, _, _ := procCreateWindowExW.Call(
			0, uintptr(unsafe.Pointer(utf16Ptr("BUTTON"))), uintptr(unsafe.Pointer(utf16Ptr(text))),
			wsChild|wsVisible|wsTabStop|bsOwnerdraw,
			0, 0, 160, 34, mw.hwnd, id, 0, 0,
		)
		return b
	}
	mw.hBtnNewSSH = createTopBtn("NEW SSH  +", idBtnNewSSH)
	mw.hBtnLocalShell = createTopBtn("NEW PWSH  +", idBtnLocalShell)
	mw.hBtnToggleList = createTopBtn("HIDE LIST", idBtnToggleList)

	// Profiles ListBox
	mw.hList, _, _ = procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(utf16Ptr("LISTBOX"))), 0,
		wsChild|wsVisible|wsVScroll|lbsNotify|wsTabStop,
		28, 112, 420, 440, mw.hwnd, idListProfiles, 0, 0,
	)
	procSendMessageW.Call(mw.hList, 0x0030, mw.normalFont, 1)

	// Right Panel
	mw.hRightPanel, _, _ = procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(utf16Ptr("TerminalLibghostyPanel"))), 0,
		wsChild|wsVisible|wsClipChildren,
		480, 112, 475, 440, mw.hwnd, 0, 0, 0,
	)
	procSetWindowLongPtrW.Call(mw.hRightPanel, uintptr(gwlpUserData), uintptr(unsafe.Pointer(mw)))

	// Tab strip and tab content area inside the right panel
	mw.hTabStrip, _, _ = procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(utf16Ptr(tabStripClassName))), 0,
		wsChild|wsVisible, 0, 0, 400, 40, mw.hRightPanel, 0, 0, 0,
	)
	procSetWindowLongPtrW.Call(mw.hTabStrip, uintptr(gwlpUserData), uintptr(unsafe.Pointer(mw)))

	mw.hConfigPanel, _, _ = procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(utf16Ptr(panelWindowClassName))), 0,
		wsChild|wsVisible|wsClipChildren, 0, 40, 400, 500, mw.hRightPanel, 0, 0, 0,
	)
	procSetWindowLongPtrW.Call(mw.hConfigPanel, uintptr(gwlpUserData), uintptr(unsafe.Pointer(mw)))

	// Form fields inside the SSH config tab
	createField := func(caption string, y int, id uintptr) (uintptr, uintptr) {
		hLbl, _, _ := procCreateWindowExW.Call(
			0, uintptr(unsafe.Pointer(utf16Ptr("STATIC"))), uintptr(unsafe.Pointer(utf16Ptr(caption))),
			wsChild|wsVisible, 24, uintptr(y), 420, 22, mw.hConfigPanel, 0, 0, 0,
		)
		procSendMessageW.Call(hLbl, 0x0030, mw.subFont, 1)

		hEdit, _, _ := procCreateWindowExW.Call(
			0x00000200, // WS_EX_CLIENTEDGE
			uintptr(unsafe.Pointer(utf16Ptr("EDIT"))), 0,
			wsChild|wsVisible|wsTabStop|esAutohscroll, 24, uintptr(y+25), 420, 30, mw.hConfigPanel, id, 0, 0,
		)
		procSendMessageW.Call(hEdit, 0x0030, mw.normalFont, 1)
		return hLbl, hEdit
	}

	mw.hLblName, mw.hEditName = createField("CONNECTION NAME", 20, idEditName)
	mw.hLblTarget, mw.hEditTarget = createField("SSH ADDRESS (user@host[:port])", 95, idEditTarget)
	mw.hLblPort, mw.hEditPort = createField("PORT OVERRIDE (OPTIONAL)", 170, idEditPort)
	mw.hLblIdentity, mw.hEditIdent = createField("PRIVATE KEY FILE (OPTIONAL)", 245, idEditIdentity)

	// Buttons inside the SSH config tab
	createBtn := func(text string, x, y, w, h int, id uintptr) uintptr {
		b, _, _ := procCreateWindowExW.Call(
			0, uintptr(unsafe.Pointer(utf16Ptr("BUTTON"))), uintptr(unsafe.Pointer(utf16Ptr(text))),
			wsChild|wsVisible|wsTabStop|bsOwnerdraw,
			uintptr(x), uintptr(y), uintptr(w), uintptr(h),
			mw.hConfigPanel, id, 0, 0,
		)
		return b
	}

	mw.hBtnSave = createBtn("SAVE CONNECTION", 24, 340, 190, 42, idBtnSave)
	mw.hBtnConnect = createBtn("CONNECT  →", 232, 340, 190, 42, idBtnConnect)
	mw.hBtnDelete = createBtn("DELETE", 24, 395, 130, 36, idBtnDelete)

	mw.tabs = []*sessionTab{{kind: tabConfig, title: "SSH CONFIG", container: mw.hConfigPanel}}
	mw.activeTab = 0
}

func (mw *MainWindow) layout() {
	var r rect
	procGetClientRect.Call(mw.hwnd, uintptr(unsafe.Pointer(&r)))
	width := int(r.Right - r.Left)
	height := int(r.Bottom - r.Top)
	if width <= 0 || height <= 0 {
		return
	}

	listW := 420
	if width < 900 {
		listW = (width - 60) / 2
	}
	contentH := height - 112
	if contentH < 300 {
		contentH = 300
	}

	// Top bar buttons, right aligned: NEW SSH, NEW PWSH, HIDE/SHOW LIST
	btnY, btnH := 24, 34
	const (
		hideW = 160
		pwshW = 170
		sshW  = 150
		gapW  = 12
	)
	x := width - 28 - hideW
	if x < 28 {
		x = 28
	}
	procMoveWindow.Call(mw.hBtnToggleList, uintptr(x), uintptr(btnY), hideW, uintptr(btnH), 1)
	x -= gapW + pwshW
	if x < 28 {
		x = 28
	}
	procMoveWindow.Call(mw.hBtnLocalShell, uintptr(x), uintptr(btnY), pwshW, uintptr(btnH), 1)
	x -= gapW + sshW
	if x < 28 {
		x = 28
	}
	procMoveWindow.Call(mw.hBtnNewSSH, uintptr(x), uintptr(btnY), sshW, uintptr(btnH), 1)

	headerW := x - 28 - 20
	if headerW < 200 {
		headerW = 200
	}
	procMoveWindow.Call(mw.hHeader, 28, 20, uintptr(headerW), 40, 1)
	procMoveWindow.Call(mw.hSub, 31, 65, uintptr(headerW), 25, 1)

	rightX := 28
	rightW := width - 56
	if !mw.listCollapsed {
		// Left ListBox
		procMoveWindow.Call(mw.hList, 28, 112, uintptr(listW), uintptr(contentH), 1)
		rightX = 28 + listW + 30
		rightW = width - rightX - 28
	}
	if rightW < 350 {
		rightW = 350
	}

	// Right Panel with tab strip and tab content
	procMoveWindow.Call(mw.hRightPanel, uintptr(rightX), 112, uintptr(rightW), uintptr(contentH), 1)
	procMoveWindow.Call(mw.hTabStrip, 0, 0, uintptr(rightW), uintptr(tabStripHeight), 1)
	tabContentH := contentH - tabStripHeight
	if tabContentH < 100 {
		tabContentH = 100
	}
	for _, t := range mw.tabs {
		procMoveWindow.Call(t.container, 0, uintptr(tabStripHeight), uintptr(rightW), uintptr(tabContentH), 1)
	}
}

func (mw *MainWindow) loadAndRefreshProfiles() {
	mw.profiles = LoadProfiles()
	procSendMessageW.Call(mw.hList, lbResetcontent, 0, 0)
	for _, p := range mw.profiles {
		displayName := fmt.Sprintf("%s  ·  %s", p.Name, p.Target)
		procSendMessageW.Call(mw.hList, lbAddstring, 0, uintptr(unsafe.Pointer(utf16Ptr(displayName))))
	}
	if len(mw.profiles) > 0 {
		procSendMessageW.Call(mw.hList, lbSetcursel, 0, 0)
		mw.selectProfile(0)
	}
}

func (mw *MainWindow) selectProfile(idx int) {
	if idx < 0 || idx >= len(mw.profiles) {
		return
	}
	p := mw.profiles[idx]
	setWindowText(mw.hEditName, p.Name)
	setWindowText(mw.hEditTarget, p.Target)
	if p.Port > 0 {
		setWindowText(mw.hEditPort, strconv.Itoa(p.Port))
	} else {
		setWindowText(mw.hEditPort, "")
	}
	setWindowText(mw.hEditIdent, p.Identity)
	mw.activateTab(0)
}

func (mw *MainWindow) setStatus(text string) {
	setWindowText(mw.hSub, text)
}

func (mw *MainWindow) handleCommand(id, code uint16) {
	switch id {
	case idListProfiles:
		switch code {
		case lbnSelchange:
			idx, _, _ := procSendMessageW.Call(mw.hList, lbGetcursel, 0, 0)
			mw.selectProfile(int(idx))
		case lbnDblclk:
			idx, _, _ := procSendMessageW.Call(mw.hList, lbGetcursel, 0, 0)
			mw.openProfileSession(int(idx))
		}

	case idBtnSave:
		target := strings.TrimSpace(getWindowText(mw.hEditTarget))
		if !targetRegex.MatchString(target) {
			mw.setStatus("Invalid target: please enter user@host[:port]")
			return
		}
		portStr := strings.TrimSpace(getWindowText(mw.hEditPort))
		port := 0
		if portStr != "" {
			p, err := strconv.Atoi(portStr)
			if err != nil || p < 1 || p > 65535 {
				mw.setStatus("Port must be between 1 and 65535")
				return
			}
			port = p
		}
		name := strings.TrimSpace(getWindowText(mw.hEditName))
		if name == "" {
			name = target
		}
		identity := strings.TrimSpace(getWindowText(mw.hEditIdent))

		updated := false
		for i := range mw.profiles {
			if mw.profiles[i].Target == target {
				mw.profiles[i].Name = name
				mw.profiles[i].Port = port
				mw.profiles[i].Identity = identity
				updated = true
				break
			}
		}
		if !updated {
			mw.profiles = append(mw.profiles, Profile{
				Name:     name,
				Target:   target,
				Port:     port,
				Identity: identity,
			})
		}
		if err := SaveProfiles(mw.profiles); err != nil {
			mw.setStatus(fmt.Sprintf("Failed to save profiles: %v", err))
			return
		}
		mw.loadAndRefreshProfiles()
		mw.setStatus("Connection profile saved.")

	case idBtnDelete:
		idx, _, _ := procSendMessageW.Call(mw.hList, lbGetcursel, 0, 0)
		i := int(idx)
		if i >= 0 && i < len(mw.profiles) {
			mw.profiles = append(mw.profiles[:i], mw.profiles[i+1:]...)
			_ = SaveProfiles(mw.profiles)
			mw.loadAndRefreshProfiles()
			mw.setStatus("Connection deleted.")
		}

	case idBtnConnect:
		target := strings.TrimSpace(getWindowText(mw.hEditTarget))
		if !targetRegex.MatchString(target) {
			mw.setStatus("Invalid target: please enter user@host[:port]")
			return
		}
		portStr := strings.TrimSpace(getWindowText(mw.hEditPort))
		port := 0
		if portStr != "" {
			p, err := strconv.Atoi(portStr)
			if err == nil && p >= 1 && p <= 65535 {
				port = p
			}
		}
		identity := strings.TrimSpace(getWindowText(mw.hEditIdent))

		mw.startSSHSession(target, identity, port, target)

	case idBtnNewSSH:
		mw.newSSHRequested()

	case idBtnLocalShell:
		mw.startPowerShellSession()

	case idBtnToggleList:
		mw.toggleList()
	}
}

func (mw *MainWindow) toggleList() {
	mw.listCollapsed = !mw.listCollapsed
	cmd := uintptr(5) // SW_SHOW
	if mw.listCollapsed {
		cmd = 0 // SW_HIDE
	}
	procShowWindow.Call(mw.hList, cmd)
	mw.layout()
	procInvalidateRect.Call(mw.hBtnToggleList, 0, 0)
	if mw.listCollapsed {
		mw.setStatus("Connection list collapsed.")
	} else {
		mw.setStatus("Connection list expanded.")
	}
}

func (mw *MainWindow) clearProfileForm() {
	setWindowText(mw.hEditName, "")
	setWindowText(mw.hEditTarget, "")
	setWindowText(mw.hEditPort, "")
	setWindowText(mw.hEditIdent, "")
	procSetFocus.Call(mw.hEditName)
}

// newSSHRequested shows the SSH config tab; when it is already active the form
// is cleared so the user can enter a new connection.
func (mw *MainWindow) newSSHRequested() {
	if mw.activeTab == 0 {
		mw.clearProfileForm()
		mw.setStatus("Enter SSH connection details.")
		return
	}
	mw.activateTab(0)
	mw.setStatus("SSH connection form.")
}

func (mw *MainWindow) Show() {
	procShowWindow.Call(mw.hwnd, 5) // SW_SHOW
	procUpdateWindow.Call(mw.hwnd)
}

func (mw *MainWindow) cleanup() {
	for _, t := range mw.tabs {
		if t.serverCmd != nil && t.serverCmd.Process != nil {
			_ = t.serverCmd.Process.Kill()
			t.serverCmd = nil
		}
	}
	if mw.bgBrush != 0 {
		procDeleteObject.Call(mw.bgBrush)
	}
	if mw.panelBrush != 0 {
		procDeleteObject.Call(mw.panelBrush)
	}
	if mw.editBrush != 0 {
		procDeleteObject.Call(mw.editBrush)
	}
	if mw.btnBrush != 0 {
		procDeleteObject.Call(mw.btnBrush)
	}
	if mw.headerFont != 0 {
		procDeleteObject.Call(mw.headerFont)
	}
	if mw.subFont != 0 {
		procDeleteObject.Call(mw.subFont)
	}
	if mw.normalFont != 0 {
		procDeleteObject.Call(mw.normalFont)
	}
	if mw.btnFont != 0 {
		procDeleteObject.Call(mw.btnFont)
	}
}

func Run() error {
	return RunWithOptions("", "", 0, "")
}

func RunWithOptions(target, identity string, port int, shell string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	mw, err := NewMainWindow()
	if err != nil {
		return err
	}
	mw.Show()

	if shell != "" {
		mw.startPowerShellSession()
	} else if target != "" {
		mw.startSSHSession(target, identity, port, target)
	}

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if m.Message == wmKeyDown && m.WParam == vkF9 {
			mw.toggleList()
			continue
		}

		// In profile manager mode, use IsDialogMessageW for tab navigation
		rDlg, _, _ := procIsDialogMessageW.Call(mw.hwnd, uintptr(unsafe.Pointer(&m)))
		if rDlg != 0 {
			continue
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}
