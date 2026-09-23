package dockerx

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/util"
)

// IptablesBroken 判断是否为 firewalld reload 冲掉 nat/DOCKER 链后的典型错误。
// 现场表现：Creating network xxx_default … SKIP DNAT … iptables: No chain/target/match by that name。
func IptablesBroken(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	if strings.Contains(s, "SKIP DNAT") {
		return true
	}
	return strings.Contains(s, "iptables") && strings.Contains(s, "No chain/target/match")
}

func dockerServiceActive() bool {
	out, err := exec.Command("systemctl", "is-active", "docker").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		return true
	}
	out, err = exec.Command("systemctl", "is-active", "docker.service").CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "active"
}

// RestartDaemon 重启 dockerd，重建 firewalld 清掉的 nat/DOCKER 链。
// Docker 未安装或未在跑时直接跳过。
func RestartDaemon() error {
	if runtime.GOOS != "linux" || !Which("docker") {
		return nil
	}
	if !dockerServiceActive() {
		return nil
	}
	util.Warnf("重启 docker，以重建 iptables 的 DOCKER 链（firewalld 变更后必须）")
	out, err := util.RunPrivileged("systemctl", "restart", "docker")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("重启 docker 失败: %s", msg)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if New().Available() {
			util.Successf("docker 已就绪")
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("docker 已重启但 daemon 30s 内未就绪")
}
