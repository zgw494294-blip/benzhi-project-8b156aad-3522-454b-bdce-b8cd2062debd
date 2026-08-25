# 石材修复试配验证工作台

本项目面向历史建筑保护团队，把石材修复材料从目标定义、配方修订、试验块养护观测、偏差整改、多轮复验到技术批准组织成一条可追溯流程。草稿阶段可受控修订验收基线，同一配方可一次登记 2 至 50 个平行试验块。系统会展示养护指标趋势预警，并依据任务阈值判断终期颜色差、吸水率、附着强度和表面缺陷；结构化送审阻断清单为空后方可审查，批准后冻结完整证据并在每次查询时核验 SHA-256 摘要。

服务由 Go 提供同源浏览器页面与 JSON API，数据保存到本地 SQLite。写操作使用 `expectedVersion` 做乐观并发控制，使用 `idempotencyKey` 防止重复提交，审计事件仅追加保存。

## 构建

要求 Go 1.22 或更高版本。

```bash
go build ./cmd/server
```

## 运行

默认仅监听高位回环地址 `127.0.0.1:19081`，默认数据库为 `data/restoration.db`：

```bash
go run ./cmd/server
```

可以显式指定回环地址与数据库路径：

```bash
go run ./cmd/server -addr=127.0.0.1:19120 -db=data/restoration.db
```

也可以通过 `PORT` 提供端口号，服务会绑定 `127.0.0.1:<PORT>`。启动后在浏览器打开对应地址即可完成建档、试配、观测、整改和批准流程。

## 测试与自检

运行全部回归测试：

```bash
go test ./...
```

运行真实 HTTP 监听的有界自检：

```bash
go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
```

自检会使用临时 SQLite 数据库，在所配置地址启动完整服务，请求 `/healthz`，通过 JSON API 创建一项临时试配任务，然后优雅关闭并删除临时数据库。

## 主要接口

- `GET /`：响应式浏览器工作台。
- `GET /healthz`：数据库连通性、结构版本、完整性和冻结不变量检查。
- `POST /api/v1/restoration-cases`：创建修复试配任务。
- `POST /api/v1/restoration-cases/{caseID}/baseline-revisions`：在尚未登记试验块的草稿阶段修订目标与验收阈值。
- `POST /api/v1/restoration-cases/{caseID}/formulas`：新增不可覆盖的配方修订。
- `POST /api/v1/restoration-cases/{caseID}/patches`：登记单个试验块，或通过 `patchCodes` 成组登记同配方试验块。
- `POST /api/v1/restoration-cases/{caseID}/patches/{patchID}/observations`：依次记录初期、稳定期和终期观测。
- `POST /api/v1/restoration-cases/{caseID}/deviations/{deviationID}/remediation`：登记偏差原因、处置、替代配方和复验块。
- `POST /api/v1/restoration-cases/{caseID}/submit-review`：在证据完整且偏差关闭后送审。
- `POST /api/v1/restoration-cases/{caseID}/review-decisions`：批准并冻结，或退回整改。
- `GET /api/v1/restoration-cases/{caseID}/timeline`：读取按时间排序的审计事件。
- `GET /api/v1/restoration-cases/{caseID}/approval`：验签并读取不可变批准快照中的阈值、配方、阶段观测、评价和偏差闭环证据。

除创建任务外，每个写请求都必须携带当前任务的 `expectedVersion`、至少 8 个字符的 `idempotencyKey` 和 `actor`。发生并发冲突时刷新任务详情后，使用新版本和新的幂等键重新提交。
