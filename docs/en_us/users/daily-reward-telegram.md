# Daily Rewards Telegram notifications

This notifier reports only the “📅 Claim Daily Rewards” task. It does not send messages for MaaEnd startup, other tasks, or the complete task chain.

## Configuration

Copy `assets/telegram-notifier.example.json` next to the MaaEnd executable, rename it to `telegram-notifier.json`, and configure:

- `bot_token`: Telegram Bot Token.
- `chat_id`: destination chat ID.
- `message_thread_id`: optional Forum Topic thread ID; keep `0` when unused.
- `disable_notification`: send silently when enabled.

The environment variables `MAAEND_TELEGRAM_BOT_TOKEN`, `MAAEND_TELEGRAM_CHAT_ID`, `MAAEND_TELEGRAM_MESSAGE_THREAD_ID`, and `MAAEND_TELEGRAM_DISABLE_NOTIFICATION` are also supported.

## Results

- Success is sent immediately after MaaFramework reports task success.
- Failure includes the last node and any available key log.
- Skipped is sent when the run ends before reaching this task.

When OneDragon forcefully terminates MaaEnd, the process cannot send a network request. The notifier persists the task state under `debug` and sends the result after OneDragon restarts MaaEnd. If there is no immediate restart, it sends the result the next time MaaEnd starts.
