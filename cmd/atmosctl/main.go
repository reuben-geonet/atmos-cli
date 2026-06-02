package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	version = "0.1.0"

	defaultBackendAddr = "127.0.0.1:6668"

	atmosAutostartFilename = "AtmosAgent.desktop"
	atmosUserService       = "atmos-agent.service"
	atmosUserServiceUnit   = "/usr/lib/systemd/user/atmos-agent.service"

	subjectStart = "tunnel.Start"
	subjectStop  = "tunnel.Stop"
)

type message struct {
	Subject         string          `json:"subject,omitempty"`
	ReplyID         string          `json:"replyID,omitempty"`
	RequestID       string          `json:"RequestID,omitempty"`
	PayloadRaw      json.RawMessage `json:"payloadRaw,omitempty"`
	PayloadIsStream bool            `json:"payloadIsStream"`
}

func main() {
	addr := flag.String("addr", defaultBackendAddr, "Atmos backend pubsub address")
	timeout := flag.Duration("timeout", 2*time.Second, "TCP connect/write timeout")
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	var err error
	switch flag.Arg(0) {
	case "version":
		if flag.NArg() != 1 {
			usage()
			os.Exit(2)
		}
		fmt.Printf("atmosctl %s\n", version)
		return
	case "vpn":
		if flag.NArg() != 2 {
			usage()
			os.Exit(2)
		}
		err = handleVPN(flag.Arg(1), *addr, *timeout)
	case "autostart":
		if flag.NArg() != 2 {
			usage()
			os.Exit(2)
		}
		err = handleAutostart(flag.Arg(1))
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [--addr %s] [--timeout 2s] version | vpn status|pause|resume | autostart status|enable|disable\n", os.Args[0], defaultBackendAddr)
}

func handleVPN(command, addr string, timeout time.Duration) error {
	subject, sendsPubsub, err := vpnCommandSubject(command)
	if err != nil {
		return err
	}
	if !sendsPubsub {
		return printStatus()
	}
	return sendSubject(addr, subject, timeout)
}

func vpnCommandSubject(command string) (string, bool, error) {
	switch command {
	case "status":
		return "", false, nil
	case "pause", "stop":
		return subjectStop, true, nil
	case "resume", "start":
		return subjectStart, true, nil
	default:
		return "", false, fmt.Errorf("unknown vpn command %q", command)
	}
}

func printStatus() error {
	state, detail, err := atmosInterfaceStatus()
	if err != nil {
		return err
	}

	if detail == "" {
		fmt.Println(state)
		return nil
	}

	fmt.Printf("%s\t%s\n", state, detail)
	return nil
}

func handleAutostart(command string) error {
	switch command {
	case "status":
		enabled, detail, err := quietAutostartStatus()
		if err != nil {
			return err
		}
		if enabled {
			fmt.Println("enabled\t" + detail)
		} else {
			fmt.Println("disabled\t" + detail)
		}
		return nil
	case "enable":
		if err := writeQuietAutostartOverride(); err != nil {
			return err
		}
		if err := runSystemctlUser("enable", atmosUserService); err != nil {
			return fmt.Errorf("autostart partially configured: wrote hidden desktop override at %s, but enabling %s failed: %w", quietAutostartPath(), atmosUserService, err)
		}
		fmt.Println("enabled")
		return nil
	case "disable":
		if err := removeQuietAutostartOverride(); err != nil {
			return err
		}
		if err := runSystemctlUser("disable", atmosUserService); err != nil {
			return fmt.Errorf("autostart partially disabled: removed desktop override at %s, but disabling %s failed: %w", quietAutostartPath(), atmosUserService, err)
		}
		fmt.Println("disabled")
		return nil
	default:
		return fmt.Errorf("unknown autostart command %q", command)
	}
}

func quietAutostartStatus() (bool, string, error) {
	overrideHidden, err := quietAutostartOverrideHidden()
	if err != nil {
		return false, "", err
	}

	serviceEnabled, err := userServiceEnabled()
	if err != nil {
		return false, "", err
	}

	enabled, detail := formatQuietAutostartStatus(overrideHidden, serviceEnabled)
	return enabled, detail, nil
}

func formatQuietAutostartStatus(overrideHidden, serviceEnabled bool) (bool, string) {
	details := []string{}
	if overrideHidden {
		details = append(details, "autostart override hidden")
	} else {
		details = append(details, "autostart override absent")
	}
	if serviceEnabled {
		details = append(details, "user service enabled")
	} else {
		details = append(details, "user service disabled")
	}

	return overrideHidden && serviceEnabled, strings.Join(details, ", ")
}

func quietAutostartOverrideHidden() (bool, error) {
	data, err := os.ReadFile(quietAutostartPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return desktopEntryHidden(string(data)), nil
}

func desktopEntryHidden(data string) bool {
	for _, line := range strings.Split(data, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "Hidden") {
			return strings.EqualFold(strings.TrimSpace(value), "true")
		}
	}
	return false
}

func userServiceEnabled() (bool, error) {
	cmd := exec.Command("systemctl", "--user", "is-enabled", atmosUserService)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(output)) == "enabled", nil
	}

	text := strings.TrimSpace(string(output))
	if strings.Contains(text, "disabled") {
		return false, nil
	}

	return false, fmt.Errorf("systemctl --user is-enabled %s failed: %w: %s", atmosUserService, err, text)
}

func writeQuietAutostartOverride() error {
	path := quietAutostartPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	content := `[Desktop Entry]
Type=Application
Name=Atmos Agent
Comment=Disabled by Atmos GNOME Shell integration; the user service starts the backend.
Exec=/usr/bin/atmos
Terminal=false
Hidden=true
`
	tmp, err := os.CreateTemp(dir, atmosAutostartFilename+".")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func removeQuietAutostartOverride() error {
	err := os.Remove(quietAutostartPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func quietAutostartPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || home == "" {
			return filepath.Join(".config", "autostart", atmosAutostartFilename)
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "autostart", atmosAutostartFilename)
}

func runSystemctlUser(args ...string) error {
	if _, err := os.Stat(atmosUserServiceUnit); err != nil {
		return fmt.Errorf("%s is not installed: %w", atmosUserServiceUnit, err)
	}

	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func atmosInterfaceStatus() (string, string, error) {
	iface, err := net.InterfaceByName("atmos")
	if err != nil {
		if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "no such network interface") {
			return "disconnected", "interface missing", nil
		}
		return "", "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", "", err
	}

	var details []string
	hasAtmosAddress := false
	for _, addr := range addrs {
		text := addr.String()
		details = append(details, text)
		if strings.HasPrefix(text, "100.65.") {
			hasAtmosAddress = true
		}
	}

	if iface.Flags&net.FlagUp == 0 {
		return "disconnected", "interface down", nil
	}
	if hasAtmosAddress {
		return "connected", strings.Join(details, ","), nil
	}
	return "unknown", strings.Join(details, ","), nil
}

func sendSubject(addr, subject string, timeout time.Duration) error {
	frame, err := buildFrame(subject)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	_, err = conn.Write(frame)
	return err
}

func buildFrame(subject string) ([]byte, error) {
	msg := message{
		Subject:         subject,
		PayloadRaw:      json.RawMessage(`{}`),
		PayloadIsStream: false,
	}

	frame, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return append(frame, 0), nil
}
