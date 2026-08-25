package main

import "testing"

func TestConfigDefaultsAndPort(t *testing.T) {
	cfg, err := parseConfig(nil, func(string) string { return "" })
	if err != nil || cfg.Address != "127.0.0.1:19081" {
		t.Fatalf("默认配置错误: %#v %v", cfg, err)
	}
	cfg, err = parseConfig(nil, func(name string) string {
		if name == "PORT" {
			return "19444"
		}
		return ""
	})
	if err != nil || cfg.Address != "127.0.0.1:19444" {
		t.Fatalf("PORT 配置错误: %#v %v", cfg, err)
	}
}

func TestUnsafeAddressRejected(t *testing.T) {
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}, func(string) string { return "" }); err == nil {
		t.Fatal("不安全监听地址未被拒绝")
	}
}
