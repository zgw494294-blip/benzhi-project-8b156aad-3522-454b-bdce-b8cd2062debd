package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type config struct {
	Address      string
	DatabasePath string
	SelfCheck    bool
}

func parseConfig(args []string, getenv func(string) string) (config, error) {
	defaultAddress := "127.0.0.1:19081"
	if port := strings.TrimSpace(getenv("PORT")); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
		}
		defaultAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(value))
	}
	set := flag.NewFlagSet("stone-restoration-trial", flag.ContinueOnError)
	address := set.String("addr", defaultAddress, "HTTP 监听地址")
	database := set.String("db", "data/restoration.db", "SQLite 数据库路径")
	selfcheck := set.Bool("selfcheck", false, "执行 HTTP 冒烟后退出")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("存在无法识别的参数: %s", strings.Join(set.Args(), " "))
	}
	if err := validateAddress(*address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*database) == "" {
		return config{}, fmt.Errorf("数据库路径不能为空")
	}
	return config{Address: *address, DatabasePath: *database, SelfCheck: *selfcheck}, nil
}

func validateAddress(address string) error {
	if strings.TrimSpace(address) == "" {
		return fmt.Errorf("监听地址不能为空")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("监听地址必须使用 host:port 格式: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return fmt.Errorf("监听地址必须明确指定回环主机，不能绑定所有网络接口")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("监听地址必须使用回环 IP")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("监听端口无效")
	}
	return nil
}
