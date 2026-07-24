//go:build windows

/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license reproduced below.
 * This means that CYGoLogger is free software and can be used for both academic
 * and commercial purposes at absolutely no cost.
 *
 * Copyright (C) 2023-2024 ShiLiang.Hao <newhaosl@163.com>, foobra<vipgs99@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do other dealings in the Software.
 */

package Appender

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32               = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole          = modkernel32.NewProc("AllocConsole")
	procFreeConsole           = modkernel32.NewProc("FreeConsole")
	procSetConsoleTitleW      = modkernel32.NewProc("SetConsoleTitleW")
	procGetStdHandle          = modkernel32.NewProc("GetStdHandle")
	procFillConsoleScreenBuffer = modkernel32.NewProc("FillConsoleScreenBuffer")
	procSetConsoleCursorPosition = modkernel32.NewProc("SetConsoleCursorPosition")
	procOutputDebugStringW   = modkernel32.NewProc("OutputDebugStringW")
)

const stdOutputHandle = 0xFFFFFFF5 // STD_OUTPUT_HANDLE = -11

type charInfo struct {
	UnicodeChar uint16
	Attributes  uint16
}

type coord struct {
	X int16
	Y int16
}

func coordVal(c coord) uintptr {
	return uintptr(uint32(uint16(c.X)) | uint32(uint16(c.Y))<<16)
}

func charInfoVal(c charInfo) uintptr {
	return uintptr(uint32(c.UnicodeChar) | uint32(c.Attributes)<<16)
}

// clearConsoleNative clears the Windows console via FillConsoleScreenBuffer and
// resets the cursor to the top-left corner.
func clearConsoleNative() {
	h, _, _ := procGetStdHandle.Call(stdOutputHandle)
	if h == 0 || h == uintptr(^uintptr(0)) {
		return
	}
	var written uint32
	ci := charInfo{UnicodeChar: ' ', Attributes: 7}
	origin := coord{X: 0, Y: 0}
	procFillConsoleScreenBuffer.Call(h, charInfoVal(ci), 1, coordVal(origin), uintptr(unsafe.Pointer(&written)))
	procSetConsoleCursorPosition.Call(h, coordVal(origin))
}

// allocConsoleWindow allocates (bShow=true) or frees (bShow=false) a dedicated
// Windows console window, mirroring C++ CYLoggerConsoleAppender's bWindow flag.
func allocConsoleWindow(bShow bool, title string) {
	if bShow {
		procAllocConsole.Call()
		if title != "" {
			procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))))
		}
	} else {
		procFreeConsole.Call()
	}
}

// debugString forwards a message to the debugger via OutputDebugStringW,
// mirroring the C++ console appender's OutputDebugString path.
func debugString(s string) {
	procOutputDebugStringW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(s))))
}
