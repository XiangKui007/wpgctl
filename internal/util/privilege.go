package util

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
)

// IsRoot 当前进程是否以 root 运行。
func IsRoot() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	u, err := user.Current()
	if err != nil {
		return os.Geteuid() == 0
	}
	return u.Uid == "0"
}

// RunPrivileged 在非 root 时尝试 sudo 执行命令（现场常见 sudo su 前先 sudo 单条命令）。
func RunPrivileged(name string, args ...string) ([]byte, error) {
	if IsRoot() {
		return exec.Command(name, args...).CombinedOutput()
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return nil, fmt.Errorf("需要 root 权限，当前用户非 root 且未找到 sudo")
	}
	sudoArgs := append([]string{name}, args...)
	return exec.Command("sudo", sudoArgs...).CombinedOutput()
}
