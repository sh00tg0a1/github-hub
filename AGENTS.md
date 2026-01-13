# Repository Guidelines

## 项目结构与模块组织
- Go 模块以 `go.mod` 为根；可执行程序放在 `cmd/ghh/`（入口 `main.go`）。
- 共享代码置于 `internal/`（优先）或 `pkg/`；配置示例在 `configs/`；脚本在 `scripts/`；测试夹具在 `testdata/`。
- 可选启用 `vendor/` 以离线构建（`go mod vendor`）。

## 构建、测试与本地运行
- 构建客户端：`go build -o bin/ghh ./cmd/ghh`
- 构建服务端：`go build -o bin/ghh-server ./cmd/ghh-server`
- 运行客户端：`go run ./cmd/ghh --help`
- 运行服务端：`GITHUB_TOKEN=... bin/ghh-server --addr :8080 --root data`
- 测试：`go test ./... -race -cover`
- 代码检查：`go vet ./...`；如配置了 `golangci-lint`，执行 `golangci-lint run`

## 代码风格与命名规范
- 统一使用 `gofmt`/`goimports`（`go fmt ./...`）；提交前保持无格式差异。
- 包名短小全小写、无下划线；导出标识符用驼峰（如 `RepoClient`）。
- 接收者命名简短（如 `c *Client`）；错误变量为 `err`，用 `%w` 包装并以 `errors.Is/As` 判断。
- 上下文参数置于首位：`ctx context.Context`；避免包名复读（no stutter）。

## 测试规范
- 使用标准库 `testing`；文件以 `_test.go` 结尾，函数为 `TestXxx/BenchmarkXxx/ExampleXxx`。
- 优先表驱动测试；覆盖率目标 ≥80%；变更性能路径时提供基准测试。
- 测试数据放在同级 `testdata/`；如使用断言库，推荐 `testify`。

## 提交与 PR 指南
- 提交遵循 Conventional Commits：`feat: ...`、`fix: ...`、`chore: ...`。
- PR 包含：动机、方案、影响面、运行与测试方式；关联 Issue；如改动行为/配置，更新 `README.md` 与 `configs/config.example.yaml`。
- 小步提交、单一职责；附关键 CLI 输出/截图有助评审。

-## 安全与配置提示
- 不提交密钥；通过环境变量或未追踪文件注入；提供 `configs/config.example.yaml` 模板。
- 配置加载：支持 `--config` 或 `GHH_CONFIG` 指定 YAML（亦兼容 JSON）；字段含 `base_url`、`token` 与 `user`。
- 默认离线安全：支持 `-mod=vendor`；对外部输入（URL、路径）进行校验与规范化。
