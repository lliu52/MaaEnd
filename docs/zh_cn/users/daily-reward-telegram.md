# 日常奖励 Telegram 截图

启用后，“📅日常奖励领取”中的每日任务奖励流程完成时，MaaEnd 会截取当前“行动手册 → 日常”页面，并通过 Telegram Bot 发送。截图保留左侧活跃度奖励栏，可用于确认 100 活跃度奖励是否已经领取。

## 配置

将 `telegram-notifier.example.json` 复制到 MaaEnd 主程序同目录并重命名为 `telegram-notifier.json`，填写：

- `bot_token`：Telegram Bot Token。
- `chat_id`：接收截图的聊天 ID。
- `message_thread_id`：可选的 Forum Topic ID；不使用时保持 `0`。
- `disable_notification`：是否静默发送。

也可以使用环境变量 `MAAEND_TELEGRAM_BOT_TOKEN`、`MAAEND_TELEGRAM_CHAT_ID`、`MAAEND_TELEGRAM_MESSAGE_THREAD_ID` 和 `MAAEND_TELEGRAM_DISABLE_NOTIFICATION`。

Telegram 配置缺失或发送失败不会改变游戏任务的执行结果；详细原因会写入 MaaEnd 日志。
