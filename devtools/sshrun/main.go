// sshrun 免交互 SSH 命令执行工具（密码认证，纯 Go）。
// 用法:
//   go run ./devtools/sshrun -h 192.168.31.58 -u root -p <密码> -c "uptime"
//   go run ./devtools/sshrun -h 192.168.31.58 -u root -p <密码> -f script.sh   # 脚本经 bash -s 执行
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	host := flag.String("h", "", "目标主机（host 或 host:port）")
	user := flag.String("u", "root", "用户名")
	pass := flag.String("p", "", "密码")
	cmd := flag.String("c", "", "要执行的命令")
	file := flag.String("f", "", "脚本文件（经 bash -s 执行，与 -c 二选一）")
	tmo := flag.Int("t", 180, "超时（秒）")
	flag.Parse()
	if *host == "" || (*cmd == "" && *file == "") {
		fmt.Fprintln(os.Stderr, "用法: sshrun -h host [-u user] -p pass (-c cmd | -f script.sh)")
		os.Exit(2)
	}

	addr := *host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "22")
	}
	cfg := &ssh.ClientConfig{
		User: *user,
		Auth: []ssh.AuthMethod{
			ssh.Password(*pass),
			ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				ans := make([]string, len(questions))
				for i := range questions {
					ans[i] = *pass
				}
				return ans, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	cli, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "SSH-DIAL-ERR:", err)
		os.Exit(3)
	}
	defer cli.Close()

	sess, err := cli.NewSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, "SSH-SESSION-ERR:", err)
		os.Exit(3)
	}
	defer sess.Close()

	run := "bash -s"
	if *file == "" {
		run = *cmd
	} else {
		b, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "READ-ERR:", err)
			os.Exit(2)
		}
		stdin, err := sess.StdinPipe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "STDIN-ERR:", err)
			os.Exit(3)
		}
		go func() {
			defer stdin.Close()
			_, _ = stdin.Write(b)
		}()
	}

	done := make(chan struct{})
	var out []byte
	var execErr error
	go func() {
		out, execErr = sess.CombinedOutput(run)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Duration(*tmo) * time.Second):
		fmt.Fprintln(os.Stderr, "TIMEOUT")
		os.Exit(4)
	}

	os.Stdout.Write(out)
	if execErr != nil {
		if ee, ok := execErr.(*ssh.ExitError); ok {
			fmt.Fprintf(os.Stderr, "\n[exit=%d]\n", ee.ExitStatus())
			os.Exit(ee.ExitStatus())
		}
		fmt.Fprintln(os.Stderr, "EXEC-ERR:", execErr)
		os.Exit(1)
	}
}
