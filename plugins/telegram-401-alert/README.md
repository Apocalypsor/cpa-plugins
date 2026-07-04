# Telegram 401 Alert

Sends a Telegram message when CPA reports an account request failure with HTTP 401.

Configure it from CPA's built-in plugin config UI. The plugin exposes `ConfigFields`,
so CPA writes the same config object to the config file.

## Config

The config fields are:

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
