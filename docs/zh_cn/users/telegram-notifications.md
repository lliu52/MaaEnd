# Telegram 任务通知

MaaEnd 可以在每个任务开始、完成或失败时，通过 Telegram Bot 向手机发送消息。通知由 Go Agent 异步发送；Telegram 网络异常只会写入日志，不会改变任务结果或阻塞游戏脚本。

## 配置

1. 在 Telegram 中通过 [@BotFather](https://t.me/BotFather) 创建 Bot，保存 Bot Token。
2. 向该 Bot 发送一条消息，然后访问 `https://api.telegram.org/bot<你的 Bot Token>/getUpdates`，从返回结果中找到 `message.chat.id`。
3. 在 MaaEnd 安装目录中复制 `telegram-notifier.example.json`，并将副本重命名为 `telegram-notifier.json`。
4. 填入 `bot_token` 和 `chat_id`，然后重新启动 MaaEnd。

```json
{
    "bot_token": "123456789:replace-with-your-bot-token",
    "chat_id": "123456789",
    "label": "My PC",
    "message_thread_id": 0,
    "disable_notification": false
}
```

- `label`：可选，用于区分运行 MaaEnd 的电脑。
- `message_thread_id`：可选，发送到 Telegram 论坛群组话题时填写话题 ID；普通私聊保持 `0`。
- `disable_notification`：设为 `true` 时使用 Telegram 静默通知。

`telegram-notifier.json` 已加入 `.gitignore`。不要把真实 Bot Token 提交到 Git 仓库或发送给其他人。

## 使用环境变量

也可以不创建配置文件，改用以下环境变量：

- `MAAEND_TELEGRAM_BOT_TOKEN`
- `MAAEND_TELEGRAM_CHAT_ID`
- `MAAEND_TELEGRAM_LABEL`（可选）
- `MAAEND_TELEGRAM_MESSAGE_THREAD_ID`（可选）
- `MAAEND_TELEGRAM_DISABLE_NOTIFICATION`（可选，`true` 或 `false`）
- `MAAEND_TELEGRAM_CONFIG`（可选，自定义配置文件路径）

环境变量优先于配置文件中的同名设置。修改后必须重启 MaaEnd，使 Go Agent 读取新配置。

## 消息内容

每条消息包含任务入口名称、Task ID、时间和可选的设备标签；完成或失败消息还会包含运行时长。任务入口名称是 MaaEnd 内部的稳定名称，例如 `DailyRewardStart`。

如果没有收到消息，请检查 MaaEnd 日志中的 `Telegram task notifications` 或 `Failed to send Telegram task notification`。常见原因包括 Bot Token 错误、Chat ID 错误、未先向 Bot 发送消息，或本机无法连接 `api.telegram.org`。
