# Telegram task notifications

MaaEnd can send a Telegram Bot message whenever a task starts, completes, or fails. The Go Agent sends notifications asynchronously. Telegram network errors are logged and never change the task result or block the game automation.

## Configuration

1. Create a bot with [@BotFather](https://t.me/BotFather) and save its Bot Token.
2. Send the bot a message, then open `https://api.telegram.org/bot<your Bot Token>/getUpdates` and find `message.chat.id` in the response.
3. Copy `telegram-notifier.example.json` in the MaaEnd installation directory and rename the copy to `telegram-notifier.json`.
4. Set `bot_token` and `chat_id`, then restart MaaEnd.

```json
{
    "bot_token": "123456789:replace-with-your-bot-token",
    "chat_id": "123456789",
    "label": "My PC",
    "message_thread_id": 0,
    "disable_notification": false
}
```

- `label`: optional label used to identify the computer running MaaEnd.
- `message_thread_id`: optional topic ID for a Telegram forum group; leave it at `0` for a normal chat.
- `disable_notification`: set to `true` to send silent Telegram notifications.

`telegram-notifier.json` is ignored by Git. Never commit a real Bot Token or share it with anyone.

## Environment variables

You can use environment variables instead of a configuration file:

- `MAAEND_TELEGRAM_BOT_TOKEN`
- `MAAEND_TELEGRAM_CHAT_ID`
- `MAAEND_TELEGRAM_LABEL` (optional)
- `MAAEND_TELEGRAM_MESSAGE_THREAD_ID` (optional)
- `MAAEND_TELEGRAM_DISABLE_NOTIFICATION` (optional, `true` or `false`)
- `MAAEND_TELEGRAM_CONFIG` (optional custom configuration path)

Environment variables override matching file settings. Restart MaaEnd after changing the configuration so the Go Agent can reload it.

## Message contents

Each message contains the task entry, Task ID, timestamp, and optional device label. Completion and failure messages also include the elapsed time. Task entries are stable MaaEnd internal names such as `DailyRewardStart`.

If messages do not arrive, search the MaaEnd logs for `Telegram task notifications` or `Failed to send Telegram task notification`. Common causes include an invalid Bot Token, an incorrect Chat ID, not messaging the bot first, or the computer being unable to reach `api.telegram.org`.
