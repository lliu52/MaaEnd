# Daily Reward Telegram Screenshot

This custom build removes the GitHub and MirrorChyan update sources from
`interface.json`. MXU will not check for, download, or install official MaaEnd
updates, preventing them from overwriting the Telegram feature.

When enabled, MaaEnd captures the current Operational Manual daily-task page after the daily task reward flow finishes and sends it through a Telegram bot. The screenshot includes the activity reward column so that the 100-point reward status can be checked directly.

## Configuration

Copy `telegram-notifier.example.json` next to the MaaEnd executable, rename it to `telegram-notifier.json`, and configure:

- `bot_token`: Telegram bot token.
- `chat_id`: destination chat ID.
- `message_thread_id`: optional forum topic ID; leave it as `0` when unused.
- `disable_notification`: whether to send silently.

The environment variables `MAAEND_TELEGRAM_BOT_TOKEN`, `MAAEND_TELEGRAM_CHAT_ID`, `MAAEND_TELEGRAM_MESSAGE_THREAD_ID`, and `MAAEND_TELEGRAM_DISABLE_NOTIFICATION` are also supported.

Missing configuration or delivery failures do not change the game task result; details are written to the MaaEnd log.
