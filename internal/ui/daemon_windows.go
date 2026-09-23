//go:build windows

package ui

// Windows 后台进程：脱离控制台、存活探测与强制结束。

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func detachProcess(cmd *exec.Cmd) {
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
		HideWindow:    true,
	}
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	const processQueryLimitedInformation = 0x1000
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = syscall.CloseHandle(h)
	return true
}

func isOurDaemonProcess(pid int) bool {
	return processAlive(pid)
}

func stopProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// Windows 上 FindProcess 会握住进程句柄；Terminate 后 OpenProcess 仍可能成功，
	// 必须以 Wait 确认退出，不能只靠 processAlive 轮询。
	if err := p.Kill(); err != nil && processAlive(pid) {
		return err
	}
	done := make(chan error, 1)
	go func() {
		_, werr := p.Wait()
		done <- werr
	}()
	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("进程 %d 未能退出", pid)
	}
}
