package firewall

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// OpenPort 防火墙当前已放行的一条规则（端口或服务名）。
type OpenPort struct {
	Spec  string `json:"spec"`            // 展示用，如 22/tcp、ssh
	Port  int    `json:"port,omitempty"`  // 能解析出数字时填写
	Proto string `json:"proto,omitempty"` // tcp / udp
	Kind  string `json:"kind,omitempty"`  // port | service
}

var rePortSpec = regexp.MustCompile(`^(\d+)(?:-(\d+))?/(tcp|udp)$`)

// ListOpenPorts 读取当前运行中的防火墙已放行端口（firewalld / ufw）。
func ListOpenPorts(st Status) []OpenPort {
	if !st.Running {
		return nil
	}
	switch st.Tool {
	case "firewalld":
		out, _ := runFirewallCmd("--list-ports")
		ports := ParseFirewalldListPorts(string(out))
		svc, _ := runFirewallCmd("--list-services")
		return mergeOpenPorts(ports, ParseFirewalldServices(string(svc)))
	case "ufw":
		out, _ := execCombined("ufw", "status")
		return ParseUfwStatus(string(out))
	default:
		return nil
	}
}

func runFirewallCmd(args ...string) ([]byte, error) {
	return exec.Command("firewall-cmd", args...).CombinedOutput()
}

func execCombined(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// ParseFirewalldListPorts 解析 `firewall-cmd --list-ports`（如 `22/tcp 8848/tcp`）。
func ParseFirewalldListPorts(s string) []OpenPort {
	var out []OpenPort
	for _, tok := range strings.Fields(s) {
		if p, ok := parsePortSpec(tok); ok {
			out = append(out, p)
		}
	}
	return out
}

// ParseFirewalldServices 解析 `firewall-cmd --list-services`。
func ParseFirewalldServices(s string) []OpenPort {
	var out []OpenPort
	for _, tok := range strings.Fields(s) {
		name := strings.TrimSpace(tok)
		if name == "" {
			continue
		}
		out = append(out, OpenPort{Spec: name, Kind: "service"})
	}
	return out
}

// ParseUfwStatus 解析 `ufw status` 中 ALLOW 行。
func ParseUfwStatus(s string) []OpenPort {
	var out []OpenPort
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Status:") || strings.HasPrefix(line, "To ") {
			continue
		}
		if !strings.Contains(strings.ToUpper(line), "ALLOW") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		spec := strings.TrimSuffix(fields[0], "(v6)")
		if p, ok := parsePortSpec(spec); ok {
			out = append(out, p)
			continue
		}
		if port, err := strconv.Atoi(spec); err == nil {
			out = append(out, OpenPort{Spec: spec + "/tcp", Port: port, Proto: "tcp", Kind: "port"})
			continue
		}
		out = append(out, OpenPort{Spec: spec, Kind: "service"})
	}
	return out
}

func parsePortSpec(spec string) (OpenPort, bool) {
	m := rePortSpec.FindStringSubmatch(strings.ToLower(strings.TrimSpace(spec)))
	if m == nil {
		return OpenPort{}, false
	}
	port, _ := strconv.Atoi(m[1])
	return OpenPort{Spec: m[1] + "/" + m[3], Port: port, Proto: m[3], Kind: "port"}, true
}

// MergeOpenPorts 按 kind+spec 去重合并两段放行列表。
func MergeOpenPorts(a, b []OpenPort) []OpenPort {
	return mergeOpenPorts(a, b)
}

func mergeOpenPorts(a, b []OpenPort) []OpenPort {
	seen := map[string]struct{}{}
	var out []OpenPort
	for _, p := range append(a, b...) {
		key := p.Kind + "|" + p.Spec
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}
