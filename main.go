package main

import (
	"C"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

var (
	magDLL                                 = windows.NewLazySystemDLL("Magnification.dll")
	kernel32                               = windows.NewLazySystemDLL("kernel32.dll")
	user32                                 = windows.NewLazySystemDLL("user32.dll")
	procSetLayeredWindowAttributes         = user32.NewProc("SetLayeredWindowAttributes")
	procRegisterHotKey                     = user32.NewProc("RegisterHotKey")
	procMagInitialize                      = magDLL.NewProc("MagInitialize")
	procMagSetWindowSource                 = magDLL.NewProc("MagSetWindowSource")
	procMagSetWindowTransform              = magDLL.NewProc("MagSetWindowTransform")
	procMagSetWindowFilterList             = magDLL.NewProc("MagSetWindowFilterList")
	zoom                           float32 = 2
	magHwnd                        win.HWND
)

type MAGTRANSFORM = [3][3]float32

func MagSetWindowFilterList(hwnd win.HWND, filterMode int32, count int32, windows *win.HWND) bool {
	res, _, _ := procMagSetWindowFilterList.Call(
		uintptr(hwnd),
		uintptr(filterMode),
		uintptr(count),
		uintptr(unsafe.Pointer(windows)),
	)
	return res != 0
}

func RegisterHotKey(hwnd win.HWND, id int32, modifier uint32, vk uint32) uintptr {
	ret, _, _ := procRegisterHotKey.Call(
		uintptr(hwnd),
		uintptr(id),
		uintptr(modifier),
		uintptr(vk),
	)
	return ret
}

func MagInitialize() bool {
	res, _, _ := procMagInitialize.Call()
	return res != 0
}

func MagSetWindowSource(hwnd win.HWND, r *win.RECT) bool {
	res, _, _ := procMagSetWindowSource.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(r)),
	)
	return res != 0
}

func MagSetWindowTransform(hwnd win.HWND, t *MAGTRANSFORM) bool {
	res, _, _ := procMagSetWindowTransform.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(t)),
	)
	return res != 0
}

func SetLayeredWindowAttributes(hwnd win.HWND, cr uint32, alpha byte, dwflags uint32) uintptr {
	ret, _, _ := procSetLayeredWindowAttributes.Call(
		uintptr(hwnd),
		uintptr(cr),
		uintptr(alpha),
		uintptr(dwflags),
	)
	return ret
}

func UpdateZoom() {
	var transform MAGTRANSFORM
	transform[0][0] = zoom
	transform[1][1] = zoom
	transform[2][2] = 1
	if !MagSetWindowTransform(magHwnd, &transform) {
		panic("transform")
	}
}

func wndProc(hwnd win.HWND, msg uint32, wparam uintptr, lparam uintptr) uintptr {
	fmt.Printf("WNDPROC\n")

	if msg == win.WM_HOTKEY {
		vk := lparam >> 16
		fmt.Printf("Hotkey! %04X %02X\n", lparam, vk)
		if vk == 0x20 { // space
			win.PostQuitMessage(0)
		} else if vk == 0x26 { // arrow up, zoom in
			zoom = zoom + 0.5
			if zoom > 20 {
				zoom = 20
			}
			UpdateZoom()
		} else if vk == 0x28 { // arrow down, zoom out
			zoom = zoom - 0.5
			if zoom < 1 {
				zoom = 1
			}
			UpdateZoom()
		}
	}

	return win.DefWindowProc(hwnd, msg, wparam, lparam)
}

func main() {
	if !MagInitialize() {
		panic("MAG init")
	}

	fHwnd := win.GetForegroundWindow()

	className := "gomag"

	var wc win.WNDCLASSEX
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.HInstance = win.GetModuleHandle(nil)
	wc.HIcon = win.LoadIcon(0, (*uint16)(unsafe.Pointer(uintptr(win.IDI_APPLICATION))))
	wc.HCursor = win.LoadCursor(0, (*uint16)(unsafe.Pointer(uintptr(win.IDC_ARROW))))
	wc.LpszClassName = syscall.StringToUTF16Ptr(className)
	wc.LpfnWndProc = syscall.NewCallback(wndProc)

	fmt.Printf("WC %+v\n", wc)

	if win.RegisterClassEx(&wc) == 0 {
		panic("register class")
	}

	w := win.GetDesktopWindow()
	fmt.Printf("%X", w)
	var r win.RECT
	win.GetWindowRect(w, &r)
	fmt.Printf("%+v", r)

	globalHwnd := win.CreateWindowEx(
		uint32(0x800a8),
		syscall.StringToUTF16Ptr(className),
		syscall.StringToUTF16Ptr("gomag"),
		uint32(0x82000000),
		r.Left,
		r.Top,
		r.Right-r.Left,
		r.Bottom-r.Top,
		w,
		0,
		0,
		nil,
	)
	SetLayeredWindowAttributes(globalHwnd, 0, 0xff, 0x2)

	magHwnd = win.CreateWindowEx(
		uint32(0),
		syscall.StringToUTF16Ptr("Magnifier"),
		syscall.StringToUTF16Ptr("MagnifierWindow"),
		uint32(0x50000001),
		r.Left,
		r.Top,
		r.Right-r.Left,
		r.Bottom-r.Top,
		globalHwnd,
		0,
		0,
		nil,
	)

	fmt.Printf("%X %X\n", globalHwnd, magHwnd)

	if !MagSetWindowFilterList(magHwnd, 0, 1, &globalHwnd) {
		panic("filter")
	}

	win.SetForegroundWindow(fHwnd)
	win.ShowWindow(globalHwnd, 1)
	win.UpdateWindow(globalHwnd)
	win.SetWindowPos(globalHwnd, win.HWND(uintptr(0xffffffff)), 0, 0, 0, 0, 0x413)

	RegisterHotKey(globalHwnd, 1, 7, 0x26) // up arrow
	RegisterHotKey(globalHwnd, 1, 7, 0x28) // down arrow
	RegisterHotKey(globalHwnd, 1, 7, 0x20) // space

	UpdateZoom()
	if !MagSetWindowSource(magHwnd, &r) {
		panic("source")
	}
	win.InvalidateRect(magHwnd, nil, true)

	win.SetTimer(globalHwnd, 0, 16, syscall.NewCallback(func() uintptr {
		var dr win.RECT
		dw := win.GetDesktopWindow()
		win.GetWindowRect(dw, &dr)

		w := dr.Right - dr.Left
		h := dr.Bottom - dr.Top

		var cursor win.POINT
		win.GetCursorPos(&cursor)

		dr.Left = int32(float32(cursor.X) - float32(w/2)/zoom)
		dr.Right = int32(float32(cursor.X) + float32(w/2)/zoom)
		dr.Top = int32(float32(cursor.Y) - float32(h/2)/zoom)
		dr.Bottom = int32(float32(cursor.Y) + float32(h/2)/zoom)

		if !MagSetWindowSource(magHwnd, &dr) {
			panic("source")
		}
		//win.InvalidateRect(magHwnd, nil, true); // not required?
		win.SetWindowPos(globalHwnd, win.HWND(uintptr(0xffffffff)), 0, 0, 0, 0, 0x413) // bring to front
		return 0
	}))

	for {
		var msg win.MSG
		win.GetMessage(&msg, 0, 0, 0)
		if msg.Message == win.WM_QUIT {
			return
		} else {
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		}
	}
}
