# Telegram 401 Alert

Sends a Telegram message when CPA reports an account request failure with HTTP 401.

Open `/v0/resource/plugins/telegram-401-alert/index.html` to configure it from
the CPA UI. The page uses CPA's plugin config API, so CPA writes the config file.

## Config

The UI writes the same fields as:

```yaml
plugins:
  configs:
    telegram-401-alert:
      enabled: true
      priority: 10
      telegram_bot_token: "123456:ABC..."
      telegram_chat_id: "123456789"
      cooldown_seconds: 1800
```

`cooldown_seconds` throttles duplicate alerts for the same provider/auth account.
