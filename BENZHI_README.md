# BENZHI_README

基于 Go 实现的stone-restoration-trial Web 项目，一款后端服务，面向历史建筑保护团队的石材修复试配验证工作台，完整实现阈值建档、不可覆盖配方修订、试验块三阶段观测、自动偏差判定、整改复验闭环、乐观并发与幂等控制、技术审查冻结、审计时间线和只读批准记录。

## 项目说明
- 项目：benzhi-project-8b156aad-3522-454b-bdce-b8cd2062debd
- 项目用途：面向历史建筑保护团队的石材修复试配验证工作台，完整实现阈值建档、不可覆盖配方修订、试验块三阶段观测、自动偏差判定、整改复验闭环、乐观并发与幂等控制、技术审查冻结、审计时间线和只读批准记录。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-8b156aad-3522-454b-bdce-b8cd2062debd-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-8b156aad-3522-454b-bdce-b8cd2062debd-arm64 linux/arm64
docker run -it benzhi-project-8b156aad-3522-454b-bdce-b8cd2062debd-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
