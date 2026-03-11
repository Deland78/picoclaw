//go:build windows

package agent

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32          = syscall.NewLazyDLL("kernel32.dll")
	procReadConsoleInput = modkernel32.NewProc("ReadConsoleInputW")
	procGetConsoleMode   = modkernel32.NewProc("GetConsoleMode")
	procSetConsoleMode   = modkernel32.NewProc("SetConsoleMode")
)

const (
	eventKey              = 0x0001
	enableProcessedInput  = 0x0001
	enableLineInput       = 0x0002
	enableEchoInput       = 0x0004
	enableMouseInput      = 0x0010
	enableQuickEditMode   = 0x0040
	enableExtendedFlags   = 0x0080
	enableVirtualTermProc = 0x0004 // output flag, already set for ANSI output

	vkReturn = 0x0D
	vkEscape = 0x1B
	vkUp     = 0x26
	vkDown   = 0x28
)

type inputRecord struct {
	eventType uint16
	_         uint16 // padding
	event     [16]byte
}

type keyEventRecord struct {
	keyDown         int32
	repeatCount     uint16
	virtualKeyCode  uint16
	virtualScanCode uint16
	unicodeChar     uint16
	controlKeyState uint32
}

type windowsPickerReader struct {
	handle  syscall.Handle
	oldMode uint32
}

func newPickerReader() (*windowsPickerReader, error) {
	h := syscall.Handle(syscall.Stdin)

	// Save current console mode
	var oldMode uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, uintptr(h), uintptr(unsafe.Pointer(&oldMode)), 0)
	if r == 0 {
		return nil, e
	}

	// Disable line input, echo, and processed input so we get raw key events
	newMode := oldMode &^ (enableLineInput | enableEchoInput | enableProcessedInput | enableMouseInput | enableQuickEditMode)
	newMode |= enableExtendedFlags
	r, _, e = syscall.Syscall(procSetConsoleMode.Addr(), 2, uintptr(h), uintptr(newMode), 0)
	if r == 0 {
		return nil, e
	}

	return &windowsPickerReader{handle: h, oldMode: oldMode}, nil
}

func (r *windowsPickerReader) ReadKey() pickerKey {
	var ir inputRecord
	var read uint32
	for {
		ret, _, _ := syscall.Syscall6(
			procReadConsoleInput.Addr(), 4,
			uintptr(r.handle),
			uintptr(unsafe.Pointer(&ir)),
			1,
			uintptr(unsafe.Pointer(&read)),
			0, 0,
		)
		if ret == 0 {
			return pickerKeyEscape // error, bail
		}

		if ir.eventType != eventKey {
			continue
		}

		ker := (*keyEventRecord)(unsafe.Pointer(&ir.event[0]))
		if ker.keyDown == 0 {
			continue // key up event, skip
		}

		switch ker.virtualKeyCode {
		case vkUp:
			return pickerKeyUp
		case vkDown:
			return pickerKeyDown
		case vkReturn:
			return pickerKeyEnter
		case vkEscape:
			return pickerKeyEscape
		}

		// Check for Ctrl+C (unicodeChar == 3)
		if ker.unicodeChar == 3 {
			return pickerKeyCtrlC
		}
	}
}

func (r *windowsPickerReader) Close() error {
	syscall.Syscall(procSetConsoleMode.Addr(), 2, uintptr(r.handle), uintptr(r.oldMode), 0)
	return nil
}
