package yggdrasil

// The darwin platform specific tun parts

import "unsafe"
import "strings"
import "strconv"
import "encoding/binary"
import "golang.org/x/sys/unix"

import water "github.com/songgao/water"

func (tun *tunDevice) setup(ifname string, addr string, mtu int) error {
	config := water.Config{DeviceType: water.TUN}
	iface, err := water.New(config)
	if err != nil {
		panic(err)
	}
	tun.iface = iface
	tun.mtu = mtu
	return tun.setupAddress(addr)
}

const SIOCAIFADDR_IN6 = 2155899162

type in6_addrlifetime struct {
	ia6t_expire    float64
	ia6t_preferred float64
	ia6t_vltime    uint32
	ia6t_pltime    uint32
}

type sockaddr_in6 struct {
	sin6_len      uint8
	sin6_family   uint8
	sin6_port     uint8
	sin6_flowinfo uint32
	sin6_addr     [8]uint16
	sin6_scope_id uint32
}

type in6_aliasreq struct {
	ifra_name       [16]byte
	ifra_addr       sockaddr_in6
	ifra_dstaddr    sockaddr_in6
	ifra_prefixmask sockaddr_in6
	ifra_flags			uint32
	ifra_lifetime   in6_addrlifetime
}

type ifreq struct {
	ifr_name				[16]byte
	ifru_mtu				uint32
}

func (tun *tunDevice) setupAddress(addr string) error {
	var fd int
	var err error

	if fd, err = unix.Socket(unix.AF_INET6, unix.SOCK_DGRAM, 0); err != nil {
		tun.core.log.Printf("Create AF_SYSTEM socket failed: %v.", err)
		return err
	}

	var ar in6_aliasreq
	copy(ar.ifra_name[:], tun.iface.Name())

	ar.ifra_prefixmask.sin6_len = uint8(unsafe.Sizeof(ar.ifra_prefixmask))
	b := make([]byte, 16); binary.LittleEndian.PutUint16(b, uint16(0xFF00))
	ar.ifra_prefixmask.sin6_addr[0] = uint16(binary.BigEndian.Uint16(b))

	ar.ifra_addr.sin6_len = uint8(unsafe.Sizeof(ar.ifra_addr))
	ar.ifra_addr.sin6_family = unix.AF_INET6
	parts := strings.Split(strings.TrimRight(addr, "/8"), ":")
	for i := 0; i < 8; i++ {
		addr, _ := strconv.ParseUint(parts[i], 16, 16)
		b := make([]byte, 16); binary.LittleEndian.PutUint16(b, uint16(addr))
		ar.ifra_addr.sin6_addr[i] = uint16(binary.BigEndian.Uint16(b))
	}

	ar.ifra_lifetime.ia6t_vltime = 0xFFFFFFFF
	ar.ifra_lifetime.ia6t_pltime = 0xFFFFFFFF

	var ir ifreq
	copy(ir.ifr_name[:], tun.iface.Name())
	ir.ifru_mtu = uint32(tun.mtu)

	tun.core.log.Printf("Interface name: %s", ar.ifra_name)
	tun.core.log.Printf("Interface IPv6: %s", addr)
	tun.core.log.Printf("Interface MTU: %d", ir.ifru_mtu)

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(SIOCAIFADDR_IN6), uintptr(unsafe.Pointer(&ar))); errno != 0 {
		err = errno
		tun.core.log.Printf("Error in SIOCAIFADDR_IN6: %v", errno)
		return err
	}

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.SIOCSIFMTU), uintptr(unsafe.Pointer(&ir))); errno != 0 {
		err = errno
		tun.core.log.Printf("Error in SIOCSIFMTU: %v", errno)
		return err
	}

	return err
}