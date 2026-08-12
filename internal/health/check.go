// Package health 提供分层启动所需的协议级健康探测。
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// Result 单次健康检查结果。
type Result struct {
	Name    string
	OK      bool
	Message string
	Elapsed time.Duration
}

// Checker 健康检查器。
type Checker struct {
	HTTPClient *http.Client
}

// New 创建默认检查器。
func New() *Checker {
	return &Checker{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// WaitHealthy 按服务规格轮询直到健康或超时。
//
// host 一般为 127.0.0.1（host 网络模式）或容器可达地址。
func (c *Checker) WaitHealthy(ctx context.Context, host string, svc config.ServiceSpec) Result {
	start := time.Now()
	timeout := time.Duration(svc.Health.TimeoutSec) * time.Second
	interval := time.Duration(svc.Health.IntervalSec) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := start.Add(timeout)

	var lastErr error
	for {
		if ctx.Err() != nil {
			return Result{Name: svc.Name, OK: false, Message: ctx.Err().Error(), Elapsed: time.Since(start)}
		}
		err := c.probe(host, svc)
		if err == nil {
			return Result{Name: svc.Name, OK: true, Message: "healthy", Elapsed: time.Since(start)}
		}
		lastErr = err
		util.Debugf("健康检查未通过 %s: %v", svc.Name, err)
		if time.Now().After(deadline) {
			return Result{
				Name:    svc.Name,
				OK:      false,
				Message: fmt.Sprintf("超时(%ds): %v", svc.Health.TimeoutSec, lastErr),
				Elapsed: time.Since(start),
			}
		}
		select {
		case <-ctx.Done():
			return Result{Name: svc.Name, OK: false, Message: ctx.Err().Error(), Elapsed: time.Since(start)}
		case <-time.After(interval):
		}
	}
}

func (c *Checker) probe(host string, svc config.ServiceSpec) error {
	switch svc.Health.Type {
	case "none":
		return nil
	case "tcp":
		addr := fmt.Sprintf("%s:%d", host, svc.Port)
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	case "http", "":
		url := fmt.Sprintf("http://%s:%d%s", host, svc.Port, svc.Health.Path)
		resp, err := c.HTTPClient.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return nil
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	default:
		return fmt.Errorf("未知健康检查类型: %s", svc.Health.Type)
	}
}

// ProbeOnce 执行一次即时探测（供 precheck 网络连通性使用）。
func (c *Checker) ProbeOnce(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
