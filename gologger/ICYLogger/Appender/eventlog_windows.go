//go:build windows

/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license.
 *
 * Copyright (C) 2023-2024 ShiLiang.Hao <newhaosl@163.com>, foobra<vipgs99@gmail.com>
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package Appender

import (
	"syscall"
	"unsafe"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

var (
	advapi32                   = syscall.NewLazyDLL("advapi32.dll")
	procRegisterEventSourceW   = advapi32.NewProc("RegisterEventSourceW")
	procReportEventW           = advapi32.NewProc("ReportEventW")
	procDeregisterEventSourceW = advapi32.NewProc("DeregisterEventSource")
)

// Windows Event Log event types (EVENTLOG_*_TYPE).
const (
	eventlogErrorType       = 0x0001
	eventlogWarningType     = 0x0002
	eventlogInformationType = 0x0004
)

// eventLogWriter is the Windows implementation of systemLogWriter, backed by
// the Windows Event Log API (advapi32.dll). It uses syscall.LazyDLL so no cgo
// toolchain is required.
type eventLogWriter struct {
	source string
}

// newSystemLogWriter creates an Event Log writer for the given source name.
func newSystemLogWriter(appName string, eLogType Core.ELogType) (systemLogWriter, error) {
	return &eventLogWriter{source: appName}, nil
}

// Write reports the message to the Application event log under the configured
// source, mapping the CYGoLogger log type to an Event Log event type.
func (e *eventLogWriter) Write(eLogType int, msg string) error {
	var et uint16
	switch Core.ELogType(eLogType) {
	case Core.LogTypeError, Core.LogTypeFatal:
		et = eventlogErrorType
	case Core.LogTypeWarn:
		et = eventlogWarningType
	default:
		et = eventlogInformationType
	}

	src := syscall.StringToUTF16Ptr(e.source)
	h, _, err := procRegisterEventSourceW.Call(0, uintptr(unsafe.Pointer(src)))
	if h == 0 {
		return err
	}
	defer procDeregisterEventSourceW.Call(h)

	msgPtr := syscall.StringToUTF16Ptr(msg)
	// ReportEventW(h, wType, wCategory, dwEventID, lpUserSid, wNumStrings,
	//               dwDataSize, lpStrings, lpRawData)
	r, _, err := procReportEventW.Call(
		h,
		uintptr(et),
		0, 0, 0,
		1, 0,
		uintptr(unsafe.Pointer(&msgPtr)),
		0,
	)
	if r == 0 {
		return err
	}
	return nil
}

func (e *eventLogWriter) Close() error { return nil }
