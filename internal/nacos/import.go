// Package nacos 通过 OpenAPI 导入/更新 Nacos 配置（方案 §6.9）。
package nacos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 导入选项。
type Options struct {
	Site      *config.SiteConfig
	ConfigDir string // 已渲染的 nacos 配置目录
	BackupDir string
}

// Result 导入结果。
type Result struct {
	Created []string
	Updated []string
	Skipped []string
}

// Run 导入渲染后的 Nacos 配置。
func Run(opts Options) (*Result, error) {
	if opts.Site == nil {
		return nil, fmt.Errorf("site 不能为空")
	}
	if opts.ConfigDir == "" || !util.DirExists(opts.ConfigDir) {
		return nil, fmt.Errorf("nacos 配置目录不存在: %s", opts.ConfigDir)
	}
	if opts.BackupDir == "" {
		opts.BackupDir = filepath.Join(filepath.Dir(opts.ConfigDir), "nacos-backup")
	}
	_ = util.EnsureDir(opts.BackupDir)

	n := opts.Site.Middleware.Nacos
	if strings.TrimSpace(n.Username) == "" {
		return nil, fmt.Errorf("middleware.nacos.username 不能为空")
	}
	if n.Password == "" {
		return nil, fmt.Errorf("middleware.nacos.password 不能为空（请检查 site.yaml 是否已保存密码）")
	}

	cli := &Client{
		Base:     fmt.Sprintf("http://%s:%d", n.Host, n.Port),
		Username: n.Username,
		Password: n.Password,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
	if err := cli.login(); err != nil {
		return nil, fmt.Errorf("Nacos 登录失败 (%s@%s): %w", cli.Username, cli.Base, err)
	}
	util.Infof("Nacos 已登录: %s namespace=%s", cli.Base, n.Namespace)

	ns := n.Namespace
	if err := cli.EnsureNamespace(ns, opts.Site.Site.Code); err != nil {
		return nil, err
	}

	res := &Result{}
	err := filepath.Walk(opts.ConfigDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".yml") &&
			!strings.HasSuffix(info.Name(), ".properties") && !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dataID := info.Name()
		group := filepath.Base(filepath.Dir(path))
		if group == "." || group == filepath.Base(opts.ConfigDir) {
			group = "DEFAULT_GROUP"
		}

		old, err := cli.GetConfig(ns, dataID, group)
		if err == nil && old == string(content) {
			res.Skipped = append(res.Skipped, dataID)
			util.Infof("配置无变化，跳过: %s@%s", dataID, group)
			return nil
		}
		if err == nil && old != "" {
			bak := filepath.Join(opts.BackupDir, group+"__"+dataID+".bak")
			_ = os.WriteFile(bak, []byte(old), 0o644)
			util.Infof("已备份旧配置: %s", bak)
			res.Updated = append(res.Updated, dataID)
		} else {
			res.Created = append(res.Created, dataID)
		}
		if err := cli.PublishConfig(ns, dataID, group, string(content)); err != nil {
			return fmt.Errorf("发布配置失败 %s: %w", dataID, err)
		}
		util.Successf("已发布: %s@%s", dataID, group)
		return nil
	})
	return res, err
}

// Client Nacos OpenAPI 客户端。
type Client struct {
	Base        string
	Username    string
	Password    string
	HTTP        *http.Client
	accessToken string
}

type loginResp struct {
	AccessToken string `json:"accessToken"`
	TokenTTL    int    `json:"tokenTtl"`
}

func (c *Client) login() error {
	form := url.Values{}
	form.Set("username", c.Username)
	form.Set("password", c.Password)
	resp, err := c.HTTP.PostForm(c.Base+"/nacos/v1/auth/login", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var lr loginResp
	if err := json.Unmarshal(body, &lr); err != nil {
		return fmt.Errorf("解析登录响应失败: %w (body=%s)", err, strings.TrimSpace(string(body)))
	}
	if lr.AccessToken == "" {
		return fmt.Errorf("未返回 accessToken (body=%s)", strings.TrimSpace(string(body)))
	}
	c.accessToken = lr.AccessToken
	return nil
}

func (c *Client) withToken(q url.Values) {
	if c.accessToken != "" {
		q.Set("accessToken", c.accessToken)
	}
}

// EnsureNamespace 命名空间不存在则创建。
func (c *Client) EnsureNamespace(namespaceID, namespaceName string) error {
	u, _ := url.Parse(c.Base + "/nacos/v1/console/namespaces")
	q := u.Query()
	c.withToken(q)
	u.RawQuery = q.Encode()

	resp, err := c.HTTP.Get(u.String())
	if err != nil {
		return fmt.Errorf("连接 Nacos 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("查询命名空间失败 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if strings.Contains(string(body), namespaceID) {
		return nil
	}
	form := url.Values{}
	form.Set("customNamespaceId", namespaceID)
	form.Set("namespaceName", namespaceName)
	form.Set("namespaceDesc", "created by wpgctl")
	c.withToken(form)
	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 300 {
		b, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("创建命名空间失败 HTTP %d: %s", resp2.StatusCode, string(b))
	}
	util.Infof("已创建 Nacos 命名空间: %s", namespaceID)
	return nil
}

// GetConfig 获取配置内容，不存在返回空串。
func (c *Client) GetConfig(namespace, dataID, group string) (string, error) {
	u, _ := url.Parse(c.Base + "/nacos/v1/cs/configs")
	q := u.Query()
	q.Set("tenant", namespace)
	q.Set("dataId", dataID)
	q.Set("group", group)
	c.withToken(q)
	u.RawQuery = q.Encode()
	resp, err := c.HTTP.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 404 || strings.TrimSpace(string(b)) == "config data not exist" {
		return "", nil
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

// PublishConfig 发布/覆盖配置。
func (c *Client) PublishConfig(namespace, dataID, group, content string) error {
	form := url.Values{}
	form.Set("tenant", namespace)
	form.Set("dataId", dataID)
	form.Set("group", group)
	form.Set("content", content)
	form.Set("type", guessType(dataID))
	c.withToken(form)
	req, err := http.NewRequest(http.MethodPost, c.Base+"/nacos/v1/cs/configs", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 || strings.TrimSpace(string(b)) == "false" {
		return fmt.Errorf("HTTP %d body=%s", resp.StatusCode, string(b))
	}
	return nil
}

func guessType(dataID string) string {
	switch {
	case strings.HasSuffix(dataID, ".yml"), strings.HasSuffix(dataID, ".yaml"):
		return "yaml"
	case strings.HasSuffix(dataID, ".json"):
		return "json"
	case strings.HasSuffix(dataID, ".properties"):
		return "properties"
	default:
		return "text"
	}
}
