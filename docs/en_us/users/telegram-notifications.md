# Telegram task notifications

MaaEnd can send Telegram Bot messages when a run starts and after every task in that run finishes. The Go Agent sends notifications asynchronously. Telegram network errors are logged and never change the task result or block the game automation.

## Configuration

1. Create a bot with [@BotFather](https://t.me/BotFather) and save its Bot Token.
2. Send the bot a message, then open `https://api.telegram.org/bot<your Bot Token>/getUpdates` and find `message.chat.id` in the response.
3. Copy `telegram-notifier.example.json` in the MaaEnd installation directory and rename the copy to `telegram-notifier.json`.
4. Set `bot_token` and `chat_id`, then restart MaaEnd.

```json
{
    "bot_token": "123456789:replace-with-your-bot-token",
    "chat_id": "123456789",
    "message_thread_id": 0,
    "disable_notification": false
}
```

- `message_thread_id`: optional topic ID for a Telegram forum group; leave it at `0` for a normal chat.
- `disable_notification`: set to `true` to send silent Telegram notifications.

`telegram-notifier.json` is ignored by Git. Never commit a real Bot Token or share it with anyone.

## Environment variables

You can use environment variables instead of a configuration file:

- `MAAEND_TELEGRAM_BOT_TOKEN`
- `MAAEND_TELEGRAM_CHAT_ID`
- `MAAEND_TELEGRAM_MESSAGE_THREAD_ID` (optional)
- `MAAEND_TELEGRAM_DISABLE_NOTIFICATION` (optional, `true` or `false`)
- `MAAEND_TELEGRAM_CONFIG` (optional custom configuration path)

Environment variables override matching file settings. Restart MaaEnd after changing the configuration so the Go Agent can reload it.

## Message contents

A run sends only two kinds of messages:

- `✅ MAA 开始运行` when the first task starts.
- A final success count such as `✅ MAA 运行完成` and `任务成功 5/5` after all tasks finish.

When any task fails, the final message uses `❌` and lists each failed internal task entry together with its last executed node. Task entries are stable MaaEnd internal names such as `DailyRewardStart`.

## Automatic updates

This custom branch removes `mirrorchyan_rid` from `interface.json`, so MXU does not automatically check for or download MaaEnd updates. To update, sync the upstream `v2` branch, then merge or rebase the Telegram feature branch and rebuild.

If messages do not arrive, search the MaaEnd logs for `Telegram task notifications` or `Failed to send Telegram task notification`. Common causes include an invalid Bot Token, an incorrect Chat ID, not messaging the bot first, or the computer being unable to reach `api.telegram.org`.
