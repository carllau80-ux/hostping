// put 经 SSH(SFTP) 上传文件并远程执行命令。
// 用法:
//
//	put -host <ip> -port <p> -user <u> -pass <p> -local <file> -remote <path> [-cmd <命令>]
//	put -host <ip> -port <p> -user <u> -pass <p> -cmd <命令>
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

type sshCli struct {
	c *gossh.Client
}

func dial(host string, port int, user, pass string, timeout time.Duration) (*sshCli, error) {
	cfg := &gossh.ClientConfig{
		User:            user,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
		// 仅密码认证：Windows OpenSSH 对 keyboard-interactive 的兼容性问题（unexpected message type 51）
		Auth: []gossh.AuthMethod{gossh.Password(pass)},
	}
	c, err := gossh.Dial("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), cfg)
	if err != nil {
		return nil, err
	}
	return &sshCli{c: c}, nil
}

func (s *sshCli) Close() {
	if s != nil && s.c != nil {
		_ = s.c.Close()
	}
}

func (s *sshCli) Run(cmd string, timeout time.Duration) (string, error) {
	sess, err := s.c.NewSession()
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
		timeout = 30 * time.Second
	}
	select {
	case r := <-done:
		return r.out, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("命令超时(%v)", timeout)
	}
}

func main() {
	host := flag.String("host", "", "目标主机")
	port := flag.Int("port", 22, "SSH 端口")
	user := flag.String("user", "", "用户")
	pass := flag.String("pass", "", "密码")
	local := flag.String("local", "", "本地文件（可空）")
	remote := flag.String("remote", "", "远端路径（可空）")
	cmd := flag.String("cmd", "", "上传后执行的命令（可空）")
	timeout := flag.Int("timeout", 30, "连接/命令超时秒")
	flag.Parse()
	if *host == "" {
		fmt.Fprintln(os.Stderr, "需要 -host")
		os.Exit(2)
	}
	cli, err := dial(*host, *port, *user, *pass, time.Duration(*timeout)*time.Second)
	if err != nil {
		fmt.Println("SSH 连接失败:", err)
		os.Exit(1)
	}
	defer cli.Close()
	fmt.Println("SSH 连接成功:", *user+"@"+*host, *port)

	if *local != "" {
		if err := upload(cli, *local, *remote); err != nil {
			fmt.Println("上传失败:", err)
			os.Exit(1)
		}
	}
	if *cmd != "" {
		fmt.Println("--- 远程执行:", *cmd)
		out, err := cli.Run(*cmd, time.Duration(*timeout)*time.Second*20)
		fmt.Println(out)
		if err != nil {
			fmt.Println("命令执行异常:", err)
		}
	}
}

func upload(cli *sshCli, local, remote string) error {
	sc, err := sftp.NewClient(cli.c)
	if err != nil {
		return err
	}
	defer sc.Close()
	src, err := os.Open(local)
	if err != nil {
		return err
	}
	defer src.Close()
	fi, err := src.Stat()
	if err != nil {
		return err
	}
	dir := filepath.ToSlash(filepath.Dir(remote))
	_ = sc.MkdirAll(dir)
	dst, err := sc.Create(remote)
	if err != nil {
		return err
	}
	defer dst.Close()
	n, err := dst.ReadFrom(src)
	if err != nil {
		return err
	}
	// 校验远端大小
	if st, err := sc.Stat(remote); err == nil {
		fmt.Printf("上传完成: %s → %s (%d/%d 字节, 远端校验 %d)\n",
			filepath.Base(local), remote, n, fi.Size(), st.Size())
	} else {
		fmt.Printf("上传完成: %s → %s (%d 字节)\n", filepath.Base(local), remote, n)
	}
	return nil
}
