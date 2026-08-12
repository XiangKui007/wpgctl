//go:build linux || darwin

package precheck

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func memTotalMB() (int, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("MemTotal 格式异常")
			}
			kb, err := strconv.Atoi(fields[1])
			if err != nil {
				return 0, err
			}
			return kb / 1024, nil
		}
	}
	return 0, fmt.Errorf("未找到 MemTotal")
}

func diskFreeGB(path string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	free := float64(st.Bavail) * float64(st.Bsize) / (1024 * 1024 * 1024)
	return free, nil
}

func checkPortListen(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// 返回 error 表示占用（由调用方解释）——这里改为用 (bool,error) 包装层
		return fmt.Errorf("占用")
	}
	_ = ln.Close()
	return nil
}
