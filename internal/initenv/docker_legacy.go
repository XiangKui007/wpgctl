package initenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// IsLegacyDockerPackage 判断目录是否为 middleware 交付包中的 docker 离线安装目录。
func IsLegacyDockerPackage(dir string) bool {
	if dir == "" {
		return false
	}
	if util.FileExists(filepath.Join(dir, "offline_install_docker.sh")) {
		return true
	}
	if util.FileExists(filepath.Join(dir, "docker.service")) {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "docker-*.tgz"))
	return len(matches) > 0
}

// installLegacyDocker 离线安装 Docker（Linux root）。优先直接执行包内 offline_install_docker.sh。
func installLegacyDocker(dir string, site *config.SiteConfig, res *Result) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("docker 安装目录无效: %w", err)
	}
	if !IsLegacyDockerPackage(abs) {
		return fmt.Errorf("目录不是有效的 Docker 离线包（需含 offline_install_docker.sh 或 docker.service / docker-*.tgz）: %s", abs)
	}

	util.Infof("使用 legacy docker 包: %s", abs)

	if dockerReady() {
		ver, _ := exec.Command("docker", "-v").CombinedOutput()
		verStr := strings.TrimSpace(string(ver))
		util.Successf("Docker 已运行，跳过安装: %s", verStr)
		res.DockerInstalled = true
		res.DockerVersion = verStr
		return nil
	}

	dataRoot := DockerDataRoot(site)
	script := filepath.Join(abs, "offline_install_docker.sh")
	if util.FileExists(script) {
		return runOfflineInstallDockerScript(abs, script, dataRoot, res)
	}
	return installLegacyDockerManual(abs, dataRoot, res)
}

func dockerReady() bool {
	ver, err := exec.Command("docker", "-v").CombinedOutput()
	if err != nil || strings.TrimSpace(string(ver)) == "" {
		return false
	}
	active, err := exec.Command("systemctl", "is-active", "docker").CombinedOutput()
	if err != nil {
		active, _ = exec.Command("systemctl", "is-active", "docker.service").CombinedOutput()
	}
	return strings.TrimSpace(string(active)) == "active"
}

func runOfflineInstallDockerScript(dir, script, dataRoot string, res *Result) error {
	util.Infof("执行 offline_install_docker.sh …")
	if util.FileExists("/etc/docker/daemon.json") {
		if err := os.Remove("/etc/docker/daemon.json"); err != nil {
			util.Warnf("无法移除旧 /etc/docker/daemon.json: %v", err)
		} else {
			util.Infof("已移除 /etc/docker/daemon.json（现场脚本不写入该文件）")
		}
	}
	_ = os.Chmod(script, 0o755)
	cmd := exec.Command("bash", script)
	cmd.Dir = dir
	scriptOut, err := cmd.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(scriptOut)), "\n") {
		if line != "" {
			util.Infof("%s", line)
		}
	}
	if err != nil {
		return fmt.Errorf("offline_install_docker.sh 失败: %w", err)
	}
	svcDest := "/etc/systemd/system/docker.service"
	if util.FileExists(svcDest) {
		if err := normalizeLegacyDockerService(svcDest, dataRoot); err != nil {
			util.Warnf("规范化 docker.service 失败: %v", err)
		} else {
			util.Infof("已设置 docker.service --graph %s", dataRoot)
			_ = exec.Command("systemctl", "daemon-reload").Run()
			_, _ = exec.Command("systemctl", "restart", "docker").CombinedOutput()
		}
	}
	verOut, err := exec.Command("docker", "-v").CombinedOutput()
	if err != nil {
		return fmt.Errorf("脚本执行后 docker 不可用: %w\n%s", err, strings.TrimSpace(string(verOut)))
	}
	verStr := strings.TrimSpace(string(verOut))
	util.Successf("Docker 就绪: %s", verStr)
	res.DockerInstalled = true
	res.DockerVersion = verStr
	return nil
}

// installLegacyDockerManual 无 offline_install_docker.sh 时的兜底安装。
func installLegacyDockerManual(abs, dataRoot string, res *Result) error {
	if err := util.EnsureDir(dataRoot); err != nil {
		return fmt.Errorf("创建 Docker 数据目录失败: %w", err)
	}

	tgz, err := findDockerTgz(abs)
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "wpgctl-docker-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	util.Infof("解压 %s …", filepath.Base(tgz))
	if out, err := exec.Command("tar", "-xf", tgz, "-C", tmpDir).CombinedOutput(); err != nil {
		return fmt.Errorf("解压 docker tgz 失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	binDir := filepath.Join(tmpDir, "docker")
	if !util.DirExists(binDir) {
		binDir = tmpDir
	}
	if err := copyDirFiles(binDir, "/usr/bin"); err != nil {
		return fmt.Errorf("安装 docker 二进制失败: %w", err)
	}

	svcSrc := filepath.Join(abs, "docker.service")
	if !util.FileExists(svcSrc) {
		return fmt.Errorf("缺少 docker.service: %s", svcSrc)
	}
	svcDest := "/etc/systemd/system/docker.service"
	if err := copyFile(svcSrc, svcDest); err != nil {
		return fmt.Errorf("安装 docker.service 失败: %w", err)
	}
	_ = os.Chmod(svcDest, 0o755)
	if err := normalizeLegacyDockerService(svcDest, dataRoot); err != nil {
		return fmt.Errorf("规范化 docker.service 失败: %w", err)
	}
	_ = os.Remove("/etc/docker/daemon.json")

	if compose, err := findDockerComposeBin(abs); err == nil {
		if err := copyFile(compose, "/usr/bin/docker-compose"); err != nil {
			return fmt.Errorf("安装 docker-compose 失败: %w", err)
		}
		_ = os.Chmod("/usr/bin/docker-compose", 0o755)
	}

	_ = exec.Command("systemctl", "reset-failed", "docker.service").Run()
	for _, c := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "start", "docker"},
		{"systemctl", "enable", "docker.service"},
	} {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if c[1] == "start" || c[1] == "docker" {
				if jout, jerr := exec.Command("journalctl", "-u", "docker.service", "-n", "15", "--no-pager").CombinedOutput(); jerr == nil {
					msg = strings.TrimSpace(msg + "\n" + string(jout))
				}
			}
			return fmt.Errorf("%s 失败: %w\n%s", strings.Join(c, " "), err, msg)
		}
	}

	verOut, err := exec.Command("docker", "-v").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker 安装后不可用: %w\n%s", err, strings.TrimSpace(string(verOut)))
	}
	verStr := strings.TrimSpace(string(verOut))
	util.Successf("Docker 就绪: %s", verStr)
	res.DockerInstalled = true
	res.DockerVersion = verStr
	return nil
}

func findDockerTgz(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "docker-*.tgz"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("目录内未找到 docker-*.tgz: %s", dir)
	}
	return matches[0], nil
}

func findDockerComposeBin(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "docker-compose-linux-*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("未找到 docker-compose-linux-*")
	}
	return matches[0], nil
}

func normalizeLegacyDockerService(servicePath, defaultGraph string) error {
	data, err := os.ReadFile(servicePath)
	if err != nil {
		return err
	}
	graph := defaultGraph
	if !strings.HasSuffix(graph, "/") {
		graph += "/"
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "ExecStart=") {
			continue
		}
		execLine := strings.TrimPrefix(trimmed, "ExecStart=")
		if p := extractDockerGraphPath(execLine); p != "" {
			graph = p
			if !strings.HasSuffix(graph, "/") {
				graph += "/"
			}
		}
		want := "ExecStart=/usr/bin/dockerd --graph " + graph
		if trimmed != want {
			lines[i] = want
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return os.WriteFile(servicePath, []byte(strings.Join(lines, "\n")), 0o644)
}

func extractDockerGraphPath(execLine string) string {
	parts := strings.Fields(execLine)
	for i, p := range parts {
		if p == "--graph" && i+1 < len(parts) {
			return parts[i+1]
		}
		if strings.HasPrefix(p, "--graph=") {
			return strings.TrimPrefix(p, "--graph=")
		}
	}
	return ""
}

func copyDirFiles(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dest := filepath.Join(destDir, e.Name())
		if err := copyFile(src, dest); err != nil {
			return err
		}
		_ = os.Chmod(dest, 0o755)
	}
	return nil
}
