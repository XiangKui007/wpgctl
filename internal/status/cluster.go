package status

import (
	"fmt"
	"strings"
	"sync"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
)

const remoteDockerPS = "docker ps -a --format '{{json .}}'"

// ClusterOptions 汇总本机 + site.yaml 中其他节点的 Docker 容器。
type ClusterOptions struct {
	Site        *config.SiteConfig
	ComposeRoot string
	SSHPassword string
	SSHKeyPath  string
}

// QueryCluster 查本机 Docker，再 SSH 查从机 docker ps；从机失败只记到对应节点，不让整页空白。
func QueryCluster(opts ClusterOptions) (ServiceListResult, error) {
	res, err := QueryServices(opts.ComposeRoot)
	if err != nil {
		res.Warning = err.Error()
		res.Services = nil
		err = nil
	}

	localName, localIP := localNodeIdentity(opts.Site)
	res.Services = annotateNode(res.Services, localName, localIP, true)
	localSnap := snapshotFromList(localName, localIP, true, res.Services, "")
	res.Nodes = []NodeSnapshot{localSnap}

	remotes := remoteNodes(opts.Site)
	if len(remotes) == 0 {
		return finishCluster(res), nil
	}

	type nodeOut struct {
		snap NodeSnapshot
		list []dockerx.ComposeService
	}
	outs := make([]nodeOut, len(remotes))
	var wg sync.WaitGroup
	for i, n := range remotes {
		i, n := i, n
		wg.Add(1)
		go func() {
			defer wg.Done()
			label := strings.TrimSpace(n.Name)
			if label == "" {
				label = n.IP
			}
			list, qerr := queryRemoteContainers(n, opts.SSHPassword, opts.SSHKeyPath)
			errText := ""
			if qerr != nil {
				errText = qerr.Error()
			}
			list = annotateNode(list, label, n.IP, false)
			outs[i] = nodeOut{
				snap: snapshotFromList(label, n.IP, false, list, errText),
				list: list,
			}
		}()
	}
	wg.Wait()

	var warns []string
	if strings.TrimSpace(res.Warning) != "" {
		warns = append(warns, strings.TrimSpace(res.Warning))
	}
	for _, o := range outs {
		res.Nodes = append(res.Nodes, o.snap)
		res.Services = append(res.Services, o.list...)
		if o.snap.Error != "" {
			warns = append(warns, fmt.Sprintf("%s (%s): %s", o.snap.Name, o.snap.IP, o.snap.Error))
		}
	}
	res.Warning = strings.Join(warns, "；")
	res.Source = "cluster"
	return finishCluster(res), nil
}

func finishCluster(res ServiceListResult) ServiceListResult {
	sortServices(res.Services)
	run, stop := summarizeServices(res.Services)
	res.Running, res.Stopped, res.Total = run, stop, len(res.Services)
	return res
}

func annotateNode(list []dockerx.ComposeService, name, ip string, local bool) []dockerx.ComposeService {
	out := make([]dockerx.ComposeService, len(list))
	for i, s := range list {
		s.Node = name
		s.NodeIP = ip
		s.Local = local
		out[i] = s
	}
	return out
}

func snapshotFromList(name, ip string, local bool, list []dockerx.ComposeService, errText string) NodeSnapshot {
	run, stop := summarizeServices(list)
	return NodeSnapshot{
		Name:    name,
		IP:      ip,
		Local:   local,
		Error:   errText,
		Running: run,
		Stopped: stop,
		Total:   len(list),
	}
}

func localNodeIdentity(site *config.SiteConfig) (name, ip string) {
	if site != nil {
		for _, n := range site.Nodes {
			if util.IsLocalIP(n.IP) {
				name = strings.TrimSpace(n.Name)
				if name == "" {
					name = "本机"
				}
				return name, strings.TrimSpace(n.IP)
			}
		}
	}
	return "本机", ""
}

func remoteNodes(site *config.SiteConfig) []config.Node {
	if site == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []config.Node
	for _, n := range site.Nodes {
		ip := strings.TrimSpace(n.IP)
		if ip == "" || util.IsLocalIP(ip) {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, n)
	}
	return out
}

func queryRemoteContainers(n config.Node, password, keyPath string) ([]dockerx.ComposeService, error) {
	sess, err := sshx.Open(n, password, keyPath, nil)
	if err != nil {
		return nil, fmt.Errorf("SSH 失败: %w", err)
	}
	defer sess.Close()
	out, err := sess.Client.Run(remoteDockerPS)
	if err != nil {
		var b strings.Builder
		full, stdin := sess.Privileged(remoteDockerPS)
		rerr := sess.Client.RunStream(full, stdin, func(line string) {
			b.WriteString(line)
			b.WriteByte('\n')
		})
		if rerr != nil {
			msg := strings.TrimSpace(out)
			if msg == "" {
				msg = err.Error()
			}
			return nil, fmt.Errorf("docker ps 失败: %s", msg)
		}
		out = b.String()
	}
	list, perr := dockerx.ParseDockerPsJSON(out)
	if perr != nil {
		return nil, fmt.Errorf("解析 docker ps: %w", perr)
	}
	return list, nil
}

func findNodeByIP(site *config.SiteConfig, ip string) (config.Node, bool) {
	ip = strings.TrimSpace(ip)
	if site == nil || ip == "" {
		return config.Node{}, false
	}
	for _, n := range site.Nodes {
		if strings.TrimSpace(n.IP) == ip {
			return n, true
		}
	}
	return config.Node{}, false
}
