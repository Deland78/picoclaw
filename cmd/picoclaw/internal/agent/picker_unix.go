//go:build !windows

package agent

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type unixPickerReader struct {
	fd      int
	oldAttr syscall.Termios
}

func newPickerReader() (*unixPickerReader, error) {
	fd := int(os.Stdin.Fd())
	var oldAttr syscall.Termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(getTermiosGet()), uintptr(unsafe.Pointer(&oldAttr)), 0, 0, 0); err != 0 {
		return nil, fmt.Errorf("tcgetattr: %w", err)
	}

	newAttr := oldAttr
	newAttr.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	newAttr.Cc[syscall.VMIN] = 1
	newAttr.Cc[syscall.VTIME] = 0
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(getTermiosSet()), uintptr(unsafe.Pointer(&newAttr)), 0, 0, 0); err != 0 {
		return nil, fmt.Errorf("tcsetattr: %w", err)
	}

	return &unixPickerReader{fd: fd, oldAttr: oldAttr}, nil
}

func (r *unixPickerReader) ReadKey() pickerKey {
	buf := make([]byte, 3)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return pickerKeyEscape
	}

	switch {
	case n == 1 && buf[0] == 3: // Ctrl+C
		return pickerKeyCtrlC
	case n == 1 && buf[0] == 27: // Escape
		return pickerKeyEscape
	case n == 1 && (buf[0] == 13 || buf[0] == 10): // Enter
		return pickerKeyEnter
	case n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'A': // Up
		return pickerKeyUp
	case n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'B': // Down
		return pickerKeyDown
	default:
		return pickerKeyNone
	}
}

func (r *unixPickerReader) Close() error {
	_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, uintptr(r.fd),
		uintptr(getTermiosSet()), uintptr(unsafe.Pointer(&r.oldAttr)), 0, 0, 0)
	return nil
}

// getTermiosGet returns the ioctl request code for TCGETS.
func getTermiosGet() uintptr {
	return syscall.TCGETS
}

// getTermiosSet returns the ioctl request code for TCSETS.
func getTermiosSet() uintptr {
	return syscall.TCSETS
}
