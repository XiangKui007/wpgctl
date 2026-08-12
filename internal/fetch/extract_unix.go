//go:build !windows

package fetch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wpg/wpgctl/internal/util"
)

func extractWithTar(archive, dest string) error {
	_ = util.EnsureDir(dest)
	cmd := exec.Command("tar", "-xzf", archive, "-C", dest, "--strip-components=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// 回退：不 strip
		cmd2 := exec.Command("tar", "-xzf", archive, "-C", dest)
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		if err2 := cmd2.Run(); err2 != nil {
			return fmt.Errorf("解压失败: %w", err2)
		}
	}
	// 若解压后多了一层目录，尝试提升
	_ = filepath.Walk
	return nil
}
