//go:build windows

package main

// Native pre-webview splash window.
//
// The perceived "dead time" at launch is the gap between double-clicking the
// exe and the moment WebView2 has booted far enough to paint frontend/index.html.
// Nothing running inside the webview can cover that gap because the webview
// isn't alive yet. This draws a plain Win32 top-most popup (brand background +
// logo + "Loading...") on its own OS thread within a few milliseconds of
// process start, and tears it down on the Wails OnDomReady hook.
//
// It is deliberately minimal: no layered/alpha window, no animation. The logo
// is composited onto the brand background at decode time so a 32bpp opaque blit
// looks correct, and a safety timer closes the window if OnDomReady never fires.

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassExW = user32.NewProc("RegisterClassExW")
	pCreateWindowExW  = user32.NewProc("CreateWindowExW")
	pDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pGetMessageW      = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessageW = user32.NewProc("DispatchMessageW")
	pPostMessageW     = user32.NewProc("PostMessageW")
	pDestroyWindow    = user32.NewProc("DestroyWindow")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pBeginPaint       = user32.NewProc("BeginPaint")
	pEndPaint         = user32.NewProc("EndPaint")
	pFillRect         = user32.NewProc("FillRect")
	pGetClientRect    = user32.NewProc("GetClientRect")
	pGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	pLoadCursorW      = user32.NewProc("LoadCursorW")
	pSetTimer         = user32.NewProc("SetTimer")
	pDrawTextW        = user32.NewProc("DrawTextW")

	pCreateSolidBrush  = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject      = gdi32.NewProc("DeleteObject")
	pStretchDIBits     = gdi32.NewProc("StretchDIBits")
	pSetStretchBltMode = gdi32.NewProc("SetStretchBltMode")
	pSetBkMode         = gdi32.NewProc("SetBkMode")
	pSetTextColor      = gdi32.NewProc("SetTextColor")
	pCreateFontW       = gdi32.NewProc("CreateFontW")
	pSelectObject      = gdi32.NewProc("SelectObject")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const (
	wsPopup        = 0x80000000
	wsVisible      = 0x10000000
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	csHRedraw      = 0x0002
	csVRedraw      = 0x0001

	idcArrow   = 32512
	smCXScreen = 0
	smCYScreen = 1

	wmDestroy = 0x0002
	wmPaint   = 0x000F
	wmClose   = 0x0010
	wmTimer   = 0x0113

	srcCopy      = 0x00CC0020
	biRGB        = 0
	dibRGBColors = 0
	halftone     = 4
	transparent  = 1

	dtCenter     = 0x0001
	dtVCenter    = 0x0004
	dtSingleLine = 0x0020

	fwSemibold        = 600
	defaultCharset    = 1
	clearTypeQuality  = 5
	splashAutoCloseMs = 12000

	// COLORREF is 0x00BBGGRR.
	splashBgColorRef   = 0x0036261B // #1B2636, matches main.go BackgroundColour
	splashTextColorRef = 0x00FFF0E3 // #E3F0FF

	splashWinW  = 420
	splashWinH  = 300
	splashLogoS = 150
	splashLogoY = 40
)

var splashBgColor = color.RGBA{R: 0x1B, G: 0x26, B: 0x36, A: 0xFF}

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type msgW struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type rect struct{ left, top, right, bottom int32 }

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]uint32
}

var (
	splashOnce      sync.Once
	splashHWND      uintptr // atomic
	splashWantClose int32   // atomic
	splashFont      uintptr

	splashImgBits []byte // BGRA, top-down, opaque
	splashImgW    int32
	splashImgH    int32
)

// i32 packs a signed 32-bit value into a uintptr for Win32 "int" parameters
// without sign-extending into the high half.
func i32(v int32) uintptr { return uintptr(uint32(v)) }

func utf16Ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

// showSplash decodes the logo and starts the splash message loop on a
// dedicated OS thread. Safe to call once; subsequent calls are ignored.
func showSplash() {
	splashOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(splashLogoPNG))
		if err != nil {
			return
		}
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(rgba, rgba.Bounds(), &image.Uniform{C: splashBgColor}, image.Point{}, draw.Src)
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Over)

		bits := make([]byte, len(rgba.Pix))
		for i := 0; i+3 < len(rgba.Pix); i += 4 {
			bits[i+0] = rgba.Pix[i+2] // B
			bits[i+1] = rgba.Pix[i+1] // G
			bits[i+2] = rgba.Pix[i+0] // R
			bits[i+3] = 0xFF
		}
		splashImgBits = bits
		splashImgW = int32(w)
		splashImgH = int32(h)

		go splashThread()
	})
}

// hideSplash asks the splash window to close. Idempotent and safe to call
// before the window has finished being created.
func hideSplash() {
	atomic.StoreInt32(&splashWantClose, 1)
	if h := atomic.LoadUintptr(&splashHWND); h != 0 {
		pPostMessageW.Call(h, uintptr(wmClose), 0, 0)
	}
}

func splashThread() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16Ptr("KashiSplashWindow")
	if className == nil {
		return
	}
	cursor, _, _ := pLoadCursorW.Call(0, uintptr(idcArrow))

	wc := wndClassExW{
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   syscall.NewCallback(splashWndProc),
		hInstance:     hInst,
		hCursor:       cursor,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if ret, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		return
	}

	cx, _, _ := pGetSystemMetrics.Call(uintptr(smCXScreen))
	cy, _, _ := pGetSystemMetrics.Call(uintptr(smCYScreen))
	x := (int32(cx) - splashWinW) / 2
	y := (int32(cy) - splashWinH) / 2

	hwnd, _, _ := pCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("Kashi Cloud"))),
		wsPopup|wsVisible,
		i32(x), i32(y), splashWinW, splashWinH,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return
	}
	atomic.StoreUintptr(&splashHWND, hwnd)

	if face := utf16Ptr("Segoe UI"); face != nil {
		splashFont, _, _ = pCreateFontW.Call(
			i32(-16), 0, 0, 0,
			fwSemibold, 0, 0, 0,
			defaultCharset, 0, 0, clearTypeQuality,
			0, uintptr(unsafe.Pointer(face)),
		)
	}

	pSetTimer.Call(hwnd, 1, splashAutoCloseMs, 0)

	// hideSplash may have been called during window creation.
	if atomic.LoadInt32(&splashWantClose) == 1 {
		pDestroyWindow.Call(hwnd)
	}

	var m msgW
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	atomic.StoreUintptr(&splashHWND, 0)
	if splashFont != 0 {
		pDeleteObject.Call(splashFont)
		splashFont = 0
	}
}

func splashWndProc(hwnd, umsg, wParam, lParam uintptr) uintptr {
	switch uint32(umsg) {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			paintSplash(hdc, hwnd)
		}
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmTimer, wmClose:
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := pDefWindowProcW.Call(hwnd, umsg, wParam, lParam)
	return ret
}

func paintSplash(hdc, hwnd uintptr) {
	var rc rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))

	if brush, _, _ := pCreateSolidBrush.Call(splashBgColorRef); brush != 0 {
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
		pDeleteObject.Call(brush)
	}

	if len(splashImgBits) > 0 && splashImgW > 0 && splashImgH > 0 {
		logoX := (rc.right - rc.left - splashLogoS) / 2
		bi := bitmapInfo{header: bitmapInfoHeader{
			biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			biWidth:       splashImgW,
			biHeight:      -splashImgH, // top-down
			biPlanes:      1,
			biBitCount:    32,
			biCompression: biRGB,
		}}
		pSetStretchBltMode.Call(hdc, halftone)
		pStretchDIBits.Call(
			hdc,
			i32(logoX), i32(splashLogoY), splashLogoS, splashLogoS,
			0, 0, uintptr(splashImgW), uintptr(splashImgH),
			uintptr(unsafe.Pointer(&splashImgBits[0])),
			uintptr(unsafe.Pointer(&bi)),
			dibRGBColors, srcCopy,
		)
	}

	caption := utf16Ptr("Loading...")
	if caption == nil {
		return
	}
	textRC := rect{
		left:   rc.left,
		top:    splashLogoY + splashLogoS + 18,
		right:  rc.right,
		bottom: splashLogoY + splashLogoS + 52,
	}
	pSetBkMode.Call(hdc, transparent)
	pSetTextColor.Call(hdc, splashTextColorRef)
	if splashFont != 0 {
		pSelectObject.Call(hdc, splashFont)
	}
	pDrawTextW.Call(
		hdc,
		uintptr(unsafe.Pointer(caption)),
		i32(-1),
		uintptr(unsafe.Pointer(&textRC)),
		dtCenter|dtSingleLine|dtVCenter,
	)
}
