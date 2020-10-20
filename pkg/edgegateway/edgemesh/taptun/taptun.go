package taptun

import (
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	cIFFTAP  = 0x0002
	cIFFNOPI = 0x1000
)

type Interface struct {
	isTAP bool
	io.ReadWriteCloser
	name string
}

type ifReq struct {
	Name  [0x10]byte
	Flags uint16
	pad   [0x28 - 0x10 - 2]byte
}

func ioctl(fd uintptr, request uintptr, argp uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(request), argp)
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}
	return nil
}

func OpenTAP(ifName string) (iface *Interface, err error) {
	var file *os.File
	if file, err = os.OpenFile("/dev/net/tun", os.O_RDWR, 0); err != nil {
		return nil, err
	}

	var flags uint16 = cIFFNOPI
	flags |= cIFFTAP
	var req ifReq
	req.Flags = flags
	copy(req.Name[:], ifName)

	// use ioctl to open the tap device
	err = ioctl(file.Fd(), syscall.TUNSETIFF, uintptr(unsafe.Pointer(&req)))
	if err != nil {
		return
	}

	return &Interface{
		isTAP:           true,
		ReadWriteCloser: file,
		name:            strings.Trim(string(req.Name[:]), "\x00"),
	}, nil
}
