//go:build !windows

package ui

// Unix 后台进程：会话分离、存活探测与 SIGTERM 停止。

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func isOurDaemonProcess(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		// 非 Linux（如 macOS 联调）读不到 /proc 时，只要进程还在就认。
		return processAlive(pid)
	}
	s := strings.ReplaceAll(string(b), "\x00", " ")
	// 子进程参数固定为 `ui --listen ...`；兼容 go run 临时二进制名不含 wpgctl。
	return strings.Contains(s, "ui") && strings.Contains(s, "--listen")
}

func stopProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	_ = p.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = p.Signal(syscall.SIGKILL)
	time.Sleep(200 * time.Millisecond)
	if processAlive(pid) {
		return fmt.Errorf("进程 %d 未能退出", pid)
	}
	return nil
}
