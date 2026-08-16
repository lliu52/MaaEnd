# 日常奖励 Telegram 通知

此通知器只报告“📅日常奖励领取”任务，不发送 MAA 启动、其他任务或整条任务链的通知。

## 配置

复制 `assets/telegram-notifier.example.json` 到 MaaEnd 主程序旁，并重命名为 `telegram-notifier.json`，然后填写：

- `bot_token`：Telegram Bot Token。
- `chat_id`：接收通知的聊天 ID。
- `message_thread_id`：可选，Forum Topic 的线程 ID；不使用时保持 `0`。
- `disable_notification`：是否静默发送。

也可以使用环境变量 `MAAEND_TELEGRAM_BOT_TOKEN`、`MAAEND_TELEGRAM_CHAT_ID`、`MAAEND_TELEGRAM_MESSAGE_THREAD_ID` 和 `MAAEND_TELEGRAM_DISABLE_NOTIFICATION`。

## 通知结果

- 成功：任务收到 MaaFramework 的成功事件后立即发送。
- 失败：任务收到失败事件后发送，并附带最后节点及可用的关键日志。
- 跳过：本次运行在执行到该任务前结束时发送。

如果 MaaEnd 被 OneDragon 强制终止，进程没有机会联网发送。通知器会把任务状态写入 `debug` 目录，并在 OneDragon 重启 MaaEnd 后补发；若没有立即重启，则在下一次 MaaEnd 启动时补发。
