package pacemaker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

// MoveResource перемещает мастер/VIP-ресурс на выбранный узел (pcs resource move)
func (c *Client) MoveResource(ctx context.Context, resourceName, targetNode string) error {
	args := []string{"resource", "move", resourceName}
	if targetNode != "" {
		args = append(args, targetNode)
	}
	return c.runPCSCommand(ctx, args...)
}

// CleanupResource сбрасывает счетчики ошибок и флаги ресурсов (pcs resource cleanup)
func (c *Client) CleanupResource(ctx context.Context, resourceName string) error {
	args := []string{"resource", "cleanup"}
	if resourceName != "" {
		args = append(args, resourceName)
	}
	return c.runPCSCommand(ctx, args...)
}

// ToggleNodeStandby переключает узел в режим обслуживания (pcs node standby / unstandby)
func (c *Client) ToggleNodeStandby(ctx context.Context, nodeName string, standby bool) error {
	action := "standby"
	if !standby {
		action = "unstandby"
	}
	args := []string{"node", action}
	if nodeName != "" {
		args = append(args, nodeName)
	}
	return c.runPCSCommand(ctx, args...)
}

func (c *Client) runPCSCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "pcs", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ошибка pcs: %v, вывод: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
