package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vishvananda/netlink"
)

// info represents the CAN interface information panel.
type info struct {
	interfaceName string
	busStatus     string
	busLoad       string
	rxErrors      uint64
	txErrors      uint64
	canErrors     uint64
}

func newInfo() info {
	return info{
		interfaceName: "can0", // Default to can0 for now
	}
}

// updateInfo fetches the latest CAN interface information.
func (i *info) updateInfo() error {
	link, err := netlink.LinkByName(i.interfaceName)
	if err != nil {
		return fmt.Errorf("could not find interface %s: %w", i.interfaceName, err)
	}

	// For now, we'll use dummy data or basic netlink stats.
	// Real bus load and detailed CAN errors require more advanced SocketCAN interaction.

	// Example of getting basic stats (bytes, packets)
	if stats := link.Attrs().Statistics; stats != nil {
		i.rxErrors = stats.RxErrors
		i.txErrors = stats.TxErrors
		// busStatus and busLoad are more complex and need further research/implementation
	}

	// Placeholder for bus status (e.g., "UP", "DOWN", "BUS-OFF")
	// This would typically come from SocketCAN error frames or specific ioctls
	i.busStatus = "UNKNOWN"

	// Placeholder for bus load (e.g., "10%")
	// This requires monitoring traffic over time
	i.busLoad = "N/A"

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