package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vishvananda/netlink"
)

// SockaddrCan is a custom struct to mimic unix.RawSockaddrCan for Linux CAN sockets.
type SockaddrCan struct {
	Family  uint16
	Ifindex int32
	_       [4]byte // padding to align to 8 bytes
}

const (
	CAN_MAX_DLEN       = 8
	CAN_RAW_ERR_FILTER = 0x4
	SOL_CAN_RAW        = 101
	CAN_ERR_MASK       = 0x1FFF
	CAN_ERR_FLAG       = 0x80000000
	CAN_ERR_BUSOFF     = 0x00000080
	CAN_ERR_CRTL       = 0x00000004
	CAN_ERR_ACK        = 0x00000040
	CAN_ERR_PROT       = 0x00000002
	CAN_ERR_TRX        = 0x00000008
	CAN_RAW            = 1 // This is a guess, might need to be adjusted
)

// info represents the CAN interface information panel.
type info struct {
	interfaceName string
	busStatus     string
	busLoad       string
	rxErrors      uint64
	txErrors      uint64
	canErrors     uint64

	totalBytes     uint64
	lastUpdateTime time.Time
	stopChan       chan struct{}
}

func newInfo() info {
	return info{
		interfaceName: "can0", // Default to can0 for now
		stopChan:      make(chan struct{}),
	}
}

// startBusLoadMonitor runs in a goroutine to calculate bus load.
func (i *info) startBusLoadMonitor() {
	link, err := netlink.LinkByName(i.interfaceName)
	if err != nil {
		// Log error or send to a channel for display
		return
	}

	sock, err := syscall.Socket(syscall.AF_CAN, syscall.SOCK_RAW, CAN_RAW)
	if err != nil {
		// Log error
		return
	}
	defer syscall.Close(sock)

	ifIdx := link.Attrs().Index
	err = syscall.Bind(sock, &syscall.SockaddrLinklayer{Ifindex: ifIdx, Protocol: CAN_RAW})
	if err != nil {
		// Log error
		return
	}

	// Set a read deadline to avoid blocking indefinitely
	tv := syscall.Timeval{Sec: 1}
	err = syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	if err != nil {
		// Log error
		return
	}

	i.totalBytes = 0
	i.lastUpdateTime = time.Now()

	ticker := time.NewTicker(time.Second) // Update bus load every second
	defer ticker.Stop()

	const CAN_MAX_DLEN = 8

	for {
		select {
		case <-i.stopChan:
			return
		case <-ticker.C:
			// Calculate bus load
			elapsed := time.Since(i.lastUpdateTime).Seconds()
			if elapsed > 0 {
				// Assuming 500 kbit/s for now (400,000 bits/s usable for data)
				// This is a rough estimate, actual bus load calculation is complex
				// and depends on bit stuffing, interframe spacing, etc.
				busSpeedBitsPerSecond := 500000.0 // 500 kbit/s
				// Approximate bits per frame (11-bit ID, 8 data bytes, plus overhead)
				// A typical CAN frame with 8 data bytes is about 111 bits on the wire.
				// Let's use a simplified calculation based on bytes for now.
				// (totalBytes * 8 bits/byte) / (busSpeedBitsPerSecond * elapsed)
				// For a more accurate calculation, we'd need to count actual frames and their overhead.
				// For now, let's just use the received bytes as a proxy.
				// A very rough estimate: 1 byte of data takes about 10-12 bits per byte due to overhead.
				// Let's assume 10 bits per byte for a quick estimate.
				load := (float64(i.totalBytes) * 10.0) / (busSpeedBitsPerSecond * elapsed)
				i.busLoad = fmt.Sprintf("%.2f%%", load*100)
			}
			i.totalBytes = 0
			i.lastUpdateTime = time.Now()
		}

		buf := make([]byte, CAN_MAX_DLEN+8) // CAN frame + can_id
		n, _, err := syscall.Recvfrom(sock, buf, syscall.MSG_DONTWAIT) // Non-blocking read
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				continue // No data, try again next tick
			}
			// Log error
			continue
		}

		if n >= 8 { // Minimum size for a CAN frame (can_id + data_length_code)
			// This is a received frame, add its data length to totalBytes
			dlc := buf[4] & 0x0F // Data Length Code is in the 5th byte, lower 4 bits
			i.totalBytes += uint64(dlc)
		}
	}
}

// updateInfo fetches the latest CAN interface information.
func (i *info) updateInfo() error {
	link, err := netlink.LinkByName(i.interfaceName)
	if err != nil {
		return fmt.Errorf("could not find interface %s: %w", i.interfaceName, err)
	}

	// Get basic interface stats
	if stats := link.Attrs().Statistics; stats != nil {
		i.rxErrors = stats.RxErrors
		i.txErrors = stats.TxErrors
	}

	// Open a raw CAN socket to receive error frames
	sock, err := syscall.Socket(syscall.AF_CAN, syscall.SOCK_RAW, CAN_RAW)
	if err != nil {
		return fmt.Errorf("failed to open CAN socket: %w", err)
	}
	defer syscall.Close(sock)

	// Bind the socket to the specific CAN interface
	ifIdx := link.Attrs().Index
	err = syscall.Bind(sock, &syscall.SockaddrLinklayer{Ifindex: ifIdx, Protocol: CAN_RAW})
	if err != nil {
		return fmt.Errorf("failed to bind CAN socket to %s: %w", i.interfaceName, err)
	}

	// Set CAN_RAW_ERR_FILTER to receive error frames
	errFilter := int(CAN_ERR_MASK)
	err = syscall.SetsockoptInt(sock, SOL_CAN_RAW, CAN_RAW_ERR_FILTER, errFilter)
	if err != nil {
		return fmt.Errorf("failed to set error filter: %w", err)
	}

	// Set a read deadline to avoid blocking indefinitely
	tv := syscall.Timeval{Sec: 1}
	err = syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	if err != nil {
		return fmt.Errorf("failed to set socket timeout: %w", err)
	}

	// Read error frames
	var canErrCount uint64
	var busStatus string = "UNKNOWN"
	for {
		buf := make([]byte, CAN_MAX_DLEN+8) // CAN frame + can_id
		n, _, err := syscall.Recvfrom(sock, buf, 0)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				break // Timeout, no more error frames for now
			}
			return fmt.Errorf("failed to read from CAN socket: %w", err)
		}

		if n < 8 { // Minimum size for a CAN frame (can_id + data_length_code)
			continue
		}

		canID := (uint32(buf[0]) << 24) | (uint32(buf[1]) << 16) | (uint32(buf[2]) << 8) | uint32(buf[3])
		if (canID & CAN_ERR_FLAG) != 0 { // Check for error frame flag
			canErrClass := canID & CAN_ERR_MASK
			canErrCount++

			switch canErrClass {
			case CAN_ERR_BUSOFF:
				busStatus = "BUS-OFF"
			case CAN_ERR_CRTL:
				// Controller problems (e.g., error passive)
				busStatus = "ERROR-PASSIVE"
			case CAN_ERR_ACK:
				// Acknowledge errors
				busStatus = "ACK-ERROR"
			case CAN_ERR_PROT:
				// Protocol errors (e.g., bit error, stuff error)
				busStatus = "PROTOCOL-ERROR"
			case CAN_ERR_TRX:
				// Transceiver errors
				busStatus = "TRANSCEIVER-ERROR"
			default:
				busStatus = "ERROR"
			}
		}
	}

	i.canErrors = canErrCount
	i.busStatus = busStatus

	return nil
}

// View renders the info panel.
func (i info) View(m Model) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CAN Interface: %s\n", i.interfaceName)
	fmt.Fprintf(&b, "Bus Status:    %s\n", i.busStatus)
	fmt.Fprintf(&b, "Bus Load:      %s\n", i.busLoad)
	fmt.Fprintf(&b, "RX Errors:     %d\n", i.rxErrors)
	fmt.Fprintf(&b, "TX Errors:     %d\n", i.txErrors)
	fmt.Fprintf(&b, "CAN Errors:    %d\n", i.canErrors)

	popup := popupStyle.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}