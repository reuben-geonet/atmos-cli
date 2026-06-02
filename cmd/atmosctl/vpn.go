package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	atmosInterfaceName = "atmos"

	reasonInterfaceMissing = "interface_missing"
	reasonInterfaceDown    = "interface_down"
	reasonNoAtmosAddress   = "no_atmos_address"
)

type vpnStatus struct {
	SchemaVersion int      `json:"schemaVersion"`
	State         string   `json:"state"`
	Interface     string   `json:"interface"`
	Addresses     []string `json:"addresses"`
	Reason        string   `json:"reason,omitempty"`
	Service       string   `json:"service"`
	ServiceActive bool     `json:"serviceActive"`
	ServiceState  string   `json:"serviceState"`
}

type vpnCommand struct {
	subject     string
	canonical   string
	sendsPubsub bool
}

func handleVPN(command, addr string, timeout time.Duration, jsonOutput bool) error {
	info, err := vpnCommandInfo(command)
	if err != nil {
		return err
	}
	if !info.sendsPubsub {
		return printVPNStatus(jsonOutput)
	}
	if err := sendSubject(addr, info.subject, timeout); err != nil {
		return err
	}
	return printCommandResult(jsonOutput, info.canonical, "")
}

func vpnCommandSubject(command string) (string, bool, error) {
	info, err := vpnCommandInfo(command)
	return info.subject, info.sendsPubsub, err
}

func vpnCommandInfo(command string) (vpnCommand, error) {
	switch command {
	case "status":
		return vpnCommand{canonical: "vpn.status"}, nil
	case "pause", "stop":
		return vpnCommand{
			subject:     subjectStop,
			canonical:   "vpn.pause",
			sendsPubsub: true,
		}, nil
	case "resume", "start":
		return vpnCommand{
			subject:     subjectStart,
			canonical:   "vpn.resume",
			sendsPubsub: true,
		}, nil
	default:
		return vpnCommand{}, fmt.Errorf("unknown vpn command %q", command)
	}
}

func printVPNStatus(jsonOutput bool) error {
	status, err := atmosInterfaceStatus()
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(status)
	}

	fmt.Println(formatVPNStatusText(status))
	return nil
}

func formatVPNStatusText(status vpnStatus) string {
	details := vpnStatusDetails(status)
	if len(details) == 0 {
		return vpnStatusTextState(status)
	}

	return fmt.Sprintf("%s\t%s", vpnStatusTextState(status), strings.Join(details, ", "))
}

func vpnStatusTextState(status vpnStatus) string {
	if status.ServiceActive {
		return status.State
	}
	return "agent-" + status.ServiceState
}

func vpnStatusDetails(status vpnStatus) []string {
	var details []string
	details = append(details, "service "+status.ServiceState)
	switch status.Reason {
	case reasonInterfaceMissing:
		details = append(details, "interface missing")
	case reasonInterfaceDown:
		details = append(details, "interface down")
	}
	if status.Reason != reasonInterfaceDown && len(status.Addresses) > 0 {
		details = append(details, "addresses "+strings.Join(status.Addresses, ","))
	}
	return details
}

func atmosInterfaceStatus() (vpnStatus, error) {
	service, err := userServiceActive()
	if err != nil {
		return vpnStatus{}, err
	}

	iface, err := net.InterfaceByName(atmosInterfaceName)
	if err != nil {
		if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "no such network interface") {
			return newVPNStatus("disconnected", nil, reasonInterfaceMissing, service), nil
		}
		return vpnStatus{}, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return vpnStatus{}, err
	}

	var addressTexts []string
	for _, addr := range addrs {
		addressTexts = append(addressTexts, addr.String())
	}

	return classifyAtmosInterfaceStatus(iface.Flags, addressTexts, service), nil
}

func classifyAtmosInterfaceStatus(flags net.Flags, addresses []string, service serviceActivity) vpnStatus {
	if flags&net.FlagUp == 0 {
		return newVPNStatus("disconnected", addresses, reasonInterfaceDown, service)
	}

	for _, address := range addresses {
		if strings.HasPrefix(address, "100.65.") {
			return newVPNStatus("connected", addresses, "", service)
		}
	}

	return newVPNStatus("unknown", addresses, reasonNoAtmosAddress, service)
}

func newVPNStatus(state string, addresses []string, reason string, service serviceActivity) vpnStatus {
	if addresses == nil {
		addresses = []string{}
	}
	if service.state == "" {
		service.state = "unknown"
	}
	return vpnStatus{
		SchemaVersion: schemaVersion,
		State:         state,
		Interface:     atmosInterfaceName,
		Addresses:     addresses,
		Reason:        reason,
		Service:       atmosUserService,
		ServiceActive: service.active,
		ServiceState:  service.state,
	}
}
