//go:build windows

package fetch

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/wpg/wpgctl/internal/util"
)

func extractWithTar(archive, dest string) error {
	_ = util.EnsureDir(dest)
	// Windows 10+ tar
	cmd := exec.Command("tar", "-xzf", archive, "-C", dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("解压失败（请确认系统 tar 可用）: %w", err)
	}
	return nil
}
