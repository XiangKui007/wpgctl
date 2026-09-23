package util

import (
	"regexp"
	"strings"
)

var (
	reANSIOSC     = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	reANSICSI     = regexp.MustCompile(`\x1b\[[0-9;:=?]*[A-Za-z]?`)
	reANSICharset = regexp.MustCompile(`\x1b[()][0-9A-B]`)
	// 页面 <pre> 会把 ESC 显示成看不见的控制符，只剩 [33m；SSH 偶发也会丢掉 ESC。
	reOrphanSGR = regexp.MustCompile(`\[[0-9]{1,3}(?:;[0-9]{1,3}){0,6}m`)
)

// StripANSI 去掉终端颜色码，便于 Web 日志页阅读（docker logs 常带 \x1b[32m 等）。
func StripANSI(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsRune(s, 0x1b) {
		s = reANSIOSC.ReplaceAllString(s, "")
		s = reANSICSI.ReplaceAllString(s, "")
		s = reANSICharset.ReplaceAllString(s, "")
	}
	return reOrphanSGR.ReplaceAllString(s, "")
}
