package cpc

import (
	"context"
	"io"
	"net"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"
)

func gitValue(ctx context.Context, dir, kind string) (string, error) {
	args := []string{"rev-parse", "--verify", "HEAD"}
	if kind == "branch" {
		args = []string{"symbolic-ref", "--quiet", "--short", "HEAD"}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	data, err := cmd.Output()
	if err != nil {
		return "", problem("无法读取 Git %s（检查仓库、提交或 detached HEAD）: %w", "Cannot read Git %s (check repository, commits or detached HEAD): %w", kind, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func localIPs() ([]item, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	entries := []item{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || !ip.IsGlobalUnicast() {
				continue
			}
			family := "IPv6"
			if ip.To4() != nil {
				family = "IPv4"
			}
			entries = append(entries, item{Key: ip.String(), Title: iface.Name, Description: family})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		ipv4i, ipv4j := net.ParseIP(entries[i].Key).To4() != nil, net.ParseIP(entries[j].Key).To4() != nil
		if ipv4i != ipv4j {
			return ipv4i
		}
		if entries[i].Title != entries[j].Title {
			return entries[i].Title < entries[j].Title
		}
		return entries[i].Key < entries[j].Key
	})
	return entries, nil
}

func publicIP(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", problem("查询外网 IP 失败: %w", "Cannot fetch public IP: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", problem("外网 IP 服务返回 HTTP %d", "Public IP service returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(data))
	if net.ParseIP(value) == nil {
		return "", problem("外网 IP 服务返回无效地址", "Public IP service returned an invalid IP")
	}
	return value, nil
}
