// Package hostinsp 通过 SSH 对 Linux 主机做只读巡检采集（纯 Go：golang.org/x/crypto/ssh）。
package hostinsp

import (
	"fmt"
	"net"
	"time"

	"dbping/internal/model"

	gossh "golang.org/x/crypto/ssh"
)

// SSHClient 一个已建立的 SSH 会话封装。
type SSHClient struct {
	cli *gossh.Client
}

// Connect 建立 SSH 连接（密码 / 私钥认证；host key 不校验——巡检工具场景）。
func Connect(h *model.SSHHost, password, keyPass string) (*SSHClient, error) {
	timeout := time.Duration(h.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	cfg := &gossh.ClientConfig{
		User:            h.User,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	auths := []gossh.AuthMethod{}
	if h.AuthType == "key" && h.KeyPath != "" {
		pemBytes, err := readFileOrEmpty(h.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败: %w", err)
		}
		signer, err := makeKeySigner(pemBytes, keyPass)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		auths = append(auths, gossh.PublicKeys(signer))
	}
	if h.AuthType == "password" || len(auths) == 0 {
		// 密码认证 + keyboard-interactive（都用密码应答，兼容强制交互式认证的服务器）
		auths = append(auths,
			gossh.Password(password),
			gossh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				ans := make([]string, len(questions))
				for i := range questions {
					ans[i] = password
				}
				return ans, nil
			}),
		)
	}
	cfg.Auth = auths

	cli, err := gossh.Dial("tcp", net.JoinHostPort(h.Host, fmt.Sprintf("%d", h.Port)), cfg)
	if err != nil {
		return nil, err
	}
	return &SSHClient{cli: cli}, nil
}

// Close 关闭连接。
func (c *SSHClient) Close() {
	if c != nil && c.cli != nil {
		_ = c.cli.Close()
	}
}

// RawClient 返回底层 SSH 客户端（供 SFTP 传输等扩展使用）。
func (c *SSHClient) RawClient() *gossh.Client {
	return c.cli
}

// Run 执行一条命令（合并 stdout/stderr，超时保护）。
func (c *SSHClient) Run(cmd string, timeout time.Duration) (string, error) {
	sess, err := c.cli.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := sess.CombinedOutput(cmd)
		done <- result{string(out), err}
	}()

	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	select {
	case r := <-done:
		if r.err != nil {
			if ee, ok := r.err.(*gossh.ExitError); ok && ee.ExitStatus() == 0 {
				return r.out, nil
			}
			// 非零退出码也返回输出（很多采集命令 exit 1 属正常，如 grep 无匹配）
			return r.out, nil
		}
		return r.out, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("命令超时(%v): %s", timeout, firstLine(cmd))
	}
}
