package ssh

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct {
	User     string
	Port     int
	KeyPath  string
	Password string
}

func NewClient(user string, port int, keyPath string, password string) *Client {
	if user == "" {
		user = "postgres"
	}
	if port <= 0 {
		port = 22
	}
	return &Client{
		User:     user,
		Port:     port,
		KeyPath:  keyPath,
		Password: password,
	}
}

func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == ""
}

func (c *Client) FetchRecentLogs(ctx context.Context, host string, target string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 30
	}

	var remoteCmd string
	if strings.HasPrefix(target, "/") {
		remoteCmd = fmt.Sprintf("tail -n %d %s 2>/dev/null", lines, target)
	} else {
		remoteCmd = fmt.Sprintf("journalctl -u %s -n %d --no-pager 2>/dev/null", target, lines)
	}

	var cmd *exec.Cmd

	if isLocalHost(host) {
		cmd = exec.CommandContext(ctx, "sh", "-c", remoteCmd)
	} else {
		targetAddr := fmt.Sprintf("%s@%s", c.User, host)
		sshArgs := []string{
			"-p", fmt.Sprintf("%d", c.Port),
			"-o", "ConnectTimeout=3",
			"-o", "StrictHostKeyChecking=no",
		}

		if c.KeyPath != "" {
			sshArgs = append(sshArgs, "-i", c.KeyPath)
		}

		if c.Password != "" {
			sshArgs = append([]string{"-p", c.Password, "ssh"}, append(sshArgs, targetAddr, remoteCmd)...)
			cmd = exec.CommandContext(ctx, "sshpass", sshArgs...)
		} else {
			sshArgs = append([]string{"-o", "BatchMode=yes"}, sshArgs...)
			sshArgs = append(sshArgs, targetAddr, remoteCmd)
			cmd = exec.CommandContext(ctx, "ssh", sshArgs...)
		}
	}

	out, err := cmd.Output()
	if err != nil {
		if isLocalHost(host) {
			return nil, fmt.Errorf("local journalctl/tail failed (%s): %w", target, err)
		}
		return nil, fmt.Errorf("ssh failed for %s: %w", host, err)
	}

	var logLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			logLines = append(logLines, line)
		}
	}

	return logLines, nil
}
