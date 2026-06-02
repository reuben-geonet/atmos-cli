package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildFrame(t *testing.T) {
	frame, err := buildFrame(subjectStop)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) == 0 || frame[len(frame)-1] != 0 {
		t.Fatalf("frame does not end with NUL: %q", frame)
	}

	var msg message
	if err := json.Unmarshal(frame[:len(frame)-1], &msg); err != nil {
		t.Fatal(err)
	}

	if msg.Subject != subjectStop {
		t.Fatalf("subject = %q, want %q", msg.Subject, subjectStop)
	}
	if string(msg.PayloadRaw) != "{}" {
		t.Fatalf("payloadRaw = %s, want {}", msg.PayloadRaw)
	}
	if msg.PayloadIsStream {
		t.Fatal("payloadIsStream = true, want false")
	}
}

func TestVPNCommandSubject(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantSubject string
		wantSend    bool
		wantErr     bool
	}{
		{name: "status", command: "status"},
		{name: "pause", command: "pause", wantSubject: subjectStop, wantSend: true},
		{name: "stop alias", command: "stop", wantSubject: subjectStop, wantSend: true},
		{name: "resume", command: "resume", wantSubject: subjectStart, wantSend: true},
		{name: "start alias", command: "start", wantSubject: subjectStart, wantSend: true},
		{name: "get-state removed", command: "get-state", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, send, err := vpnCommandSubject(tt.command)
			if tt.wantErr {
				if err == nil {
					t.Fatal("err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if subject != tt.wantSubject {
				t.Fatalf("subject = %q, want %q", subject, tt.wantSubject)
			}
			if send != tt.wantSend {
				t.Fatalf("send = %t, want %t", send, tt.wantSend)
			}
		})
	}
}

func TestVPNStatusJSONContract(t *testing.T) {
	status := classifyAtmosInterfaceStatus(net.FlagUp, []string{"100.65.0.1/32"})
	assertJSON(t, status, `{"schemaVersion":1,"state":"connected","interface":"atmos","addresses":["100.65.0.1/32"]}`)

	status = newVPNStatus("disconnected", nil, reasonInterfaceMissing)
	assertJSON(t, status, `{"schemaVersion":1,"state":"disconnected","interface":"atmos","addresses":[],"reason":"interface_missing"}`)
}

func TestFormatVPNStatusText(t *testing.T) {
	tests := []struct {
		name   string
		status vpnStatus
		want   string
	}{
		{
			name:   "connected",
			status: classifyAtmosInterfaceStatus(net.FlagUp, []string{"100.65.0.1/32"}),
			want:   "connected\t100.65.0.1/32",
		},
		{
			name:   "missing",
			status: newVPNStatus("disconnected", nil, reasonInterfaceMissing),
			want:   "disconnected\tinterface missing",
		},
		{
			name:   "down",
			status: classifyAtmosInterfaceStatus(0, []string{"100.65.0.1/32"}),
			want:   "disconnected\tinterface down",
		},
		{
			name:   "unknown",
			status: classifyAtmosInterfaceStatus(net.FlagUp, []string{"10.0.0.1/32"}),
			want:   "unknown\t10.0.0.1/32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatVPNStatusText(tt.status); got != tt.want {
				t.Fatalf("formatVPNStatusText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDesktopEntryHidden(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "true", data: "Hidden=true\n", want: true},
		{name: "true with spacing and case", data: "Hidden = TRUE\n", want: true},
		{name: "false", data: "Hidden=false\n"},
		{name: "absent", data: "Type=Application\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := desktopEntryHidden(tt.data); got != tt.want {
				t.Fatalf("desktopEntryHidden() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestFormatQuietAutostartStatus(t *testing.T) {
	tests := []struct {
		name           string
		overrideHidden bool
		serviceEnabled bool
		wantEnabled    bool
		wantDetail     string
	}{
		{
			name:           "fully enabled",
			overrideHidden: true,
			serviceEnabled: true,
			wantEnabled:    true,
			wantDetail:     "autostart override hidden, user service enabled",
		},
		{
			name:           "override only",
			overrideHidden: true,
			wantDetail:     "autostart override hidden, user service disabled",
		},
		{
			name:           "service only",
			serviceEnabled: true,
			wantDetail:     "autostart override absent, user service enabled",
		},
		{
			name:       "fully disabled",
			wantDetail: "autostart override absent, user service disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, detail := formatQuietAutostartStatus(tt.overrideHidden, tt.serviceEnabled)
			if enabled != tt.wantEnabled {
				t.Fatalf("enabled = %t, want %t", enabled, tt.wantEnabled)
			}
			if detail != tt.wantDetail {
				t.Fatalf("detail = %q, want %q", detail, tt.wantDetail)
			}
		})
	}
}

func TestAutostartStatusJSONContract(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	status := buildAutostartStatus(true, false)
	want := `{"schemaVersion":1,"enabled":false,"overrideHidden":true,"serviceEnabled":false,"overridePath":"` +
		filepath.Join(configDir, "autostart", atmosAutostartFilename) +
		`","service":"atmos-agent.service"}`
	assertJSON(t, status, want)
}

func TestVersionAndCommandJSONContracts(t *testing.T) {
	assertJSON(t, versionOutput{
		SchemaVersion: schemaVersion,
		Version:       version,
	}, `{"schemaVersion":1,"version":"`+version+`"}`)

	assertJSON(t, commandResult{
		SchemaVersion: schemaVersion,
		OK:            true,
		Command:       "vpn.pause",
	}, `{"schemaVersion":1,"ok":true,"command":"vpn.pause"}`)
}

func TestWriteQuietAutostartOverride(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	if err := writeQuietAutostartOverride(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(configDir, "autostart", atmosAutostartFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	for _, want := range []string{
		"[Desktop Entry]\n",
		"Type=Application\n",
		"Name=Atmos Agent\n",
		"Exec=/usr/bin/atmos\n",
		"Terminal=false\n",
		"Hidden=true\n",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("desktop entry missing %q in:\n%s", want, content)
		}
	}
	if !desktopEntryHidden(content) {
		t.Fatal("written desktop entry is not hidden")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode = %v, want 0644", got)
	}
}

func assertJSON(t *testing.T, value any, want string) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}
}
