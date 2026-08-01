# Telegram 无日志重启测试版

此页面仅适用于 `codex/telegram-hang-test` 测试分支。正式分支不包含故意卡死任务。

## 测试任务

MXU 的“其他”分组中会出现 `🧪 Telegram 无日志卡死测试（测试版专用）`。任务启动后会：

1. 输出一次包含进程、Task ID、动作名和静默时长的诊断日志。
2. 至少 300 秒不再输出任何日志。
3. 300 秒后继续阻塞，不会自然结束，必须由 OneDragon 无日志监控终止 MaaEnd。

## 推荐任务顺序

为了同时检查不同状态，建议将任务排成：

1. 一个通常会成功的任务。
2. 可选：一个已知会失败但允许任务链继续的任务。
3. `🧪 Telegram 无日志卡死测试（测试版专用）`。
4. 至少一个普通任务，用来验证“未执行任务”列表。

OneDragon 的 `no_log_timeout_seconds` 应设为小于或等于 `300`。当前使用 `120` 即可更快完成测试。

## 预期结果

达到无日志阈值后，OneDragon 终止 MaaEnd。Go Agent 在检测到父进程退出后，会在退出前同步发送一条 Telegram 快照，其中包括：

- 已成功任务；
- 已失败任务及最后节点；
- 正在执行的卡死测试任务及最后节点；
- 尚未执行的后续任务。

Agent 每秒检查一次父进程。Telegram 请求超时为 10 秒，整个退出处理最长等待 12 秒。无论发送成功与否，OneDragon 后续重启不会被无限阻塞。

## 调试信息

请保留同一时间段的以下内容：

- OneDragon `.log` 目录中的运行日志；
- MaaEnd `debug` 目录中的 Go Agent 日志；
- 收到的 Telegram 消息截图；
- OneDragon 实际使用的 `no_log_timeout_seconds` 和重试设置。

重点搜索以下日志组件：

- `telegram-hang-test`
- `parent-watcher`
- `telegram`

正常时间线应依次出现“进入 intentional no-log hang”、OneDragon 无日志终止、`parent process has exited`、`sending interrupted Telegram snapshot synchronously` 和发送完成记录。
