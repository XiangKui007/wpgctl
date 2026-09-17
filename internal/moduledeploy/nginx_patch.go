package moduledeploy

import (
	"fmt"

	"github.com/wpg/wpgctl/internal/fetch"
)

// NginxPatchOptions nginx 模块现场部署：模块 tar load → html zip → 改 conf → compose up。
type NginxPatchOptions struct {
	Layout         NginxLayout
	ExpandArchives bool
	ProxyIPs       NginxProxyIPs
	ComposeUp      bool
}

// NginxPatchResult nginx 步骤执行摘要。
type NginxPatchResult struct {
	WebConf    string              `json:"webConf"`
	HTMLDir    string              `json:"htmlDir"`
	Steps      []string            `json:"steps"`
	ExpandHTML *fetch.ExpandResult `json:"expandHtml,omitempty"`
	ProxyNotes []string            `json:"proxyNotes"`
	Compose    *Result             `json:"compose,omitempty"`
}

// RunNginxPatch 执行 nginx 模块完整现场流程。
func RunNginxPatch(opts NginxPatchOptions) (*NginxPatchResult, error) {
	moduleDir := opts.Layout.ModuleDir
	out := &NginxPatchResult{
		WebConf: opts.Layout.WebConf,
		HTMLDir: opts.Layout.HTMLDir,
	}

	var loadedRefs []string
	if opts.ExpandArchives {
		steps, err := PrepareModuleArchives(moduleDir)
		out.Steps = append(out.Steps, steps...)
		if err != nil {
			tars, _ := findImageTars(moduleDir)
			if len(tars) == 0 {
				return out, fmt.Errorf("解压 nginx 模块失败: %w", err)
			}
			out.Steps = append(out.Steps, "模块目录无新压缩包，沿用已有 .tar")
		}

		htmlRes, err := ExpandNginxHTML(opts.Layout.HTMLDir)
		if err != nil {
			return out, err
		}
		out.ExpandHTML = htmlRes
		if htmlRes != nil && htmlRes.ZipExtracted > 0 {
			out.Steps = append(out.Steps, fmt.Sprintf("html 解压: zip=%d", htmlRes.ZipExtracted))
		} else {
			out.Steps = append(out.Steps, "html 无待解压 zip（已跳过）")
		}

		loadRes, err := Run(Options{ModuleDir: moduleDir, Load: true})
		if err != nil {
			return out, err
		}
		out.Steps = append(out.Steps, loadRes.Steps...)
		loadedRefs = append(loadedRefs, loadRes.LoadedRefs...)
	}

	notes, err := PatchHTTPWeb8877(opts.Layout.WebConf, opts.ProxyIPs)
	if err != nil {
		return out, err
	}
	out.ProxyNotes = notes

	if opts.ComposeUp {
		composeRes, err := Run(Options{
			ModuleDir:  moduleDir,
			ComposeUp:  true,
			LoadedRefs: loadedRefs,
		})
		if err != nil {
			return out, err
		}
		out.Compose = composeRes
		out.Steps = append(out.Steps, composeRes.Steps...)
	}

	return out, nil
}
