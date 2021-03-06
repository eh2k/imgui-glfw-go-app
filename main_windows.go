package app

/*
#include <windows.h>
void SetWindowIcon(HWND hwnd, int resourceId)
{
    HANDLE hbicon = LoadImage(
        GetModuleHandle(0),
        MAKEINTRESOURCE(resourceId),
        IMAGE_ICON,
        GetSystemMetrics(SM_CXICON),
        GetSystemMetrics(SM_CYICON),
        0);
	if (hbicon)
	{
		LRESULT r = SendMessage(hwnd, WM_SETICON, ICON_BIG, (LPARAM)hbicon);
	}

    HANDLE hsicon = LoadImage(
        GetModuleHandle(0),
        MAKEINTRESOURCE(100),
        IMAGE_ICON,
        GetSystemMetrics(SM_CXSMICON),
        GetSystemMetrics(SM_CYSMICON),
        0);
	if (hsicon)
	{
		LRESULT r = SendMessage(hwnd, WM_SETICON, ICON_SMALL, (LPARAM)hsicon);
	}
}
*/
import "C"

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"unsafe"
)

func SetWindowIcon(window *glfw.Window) {

	hwnd := unsafe.Pointer(window.GetWin32Window())
	C.SetWindowIcon(C.HWND(hwnd), C.int(100))
}
