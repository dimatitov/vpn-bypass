# vpn-bypass

Кроссплатформенная CLI-утилита для маршрутизации выбранных доменов и IP напрямую, в обход full-tunnel VPN.

Поддержка на старте:

- macOS
- Windows
- OpenVPN / Tunnelblick / другие VPN-клиенты, использующие системную таблицу маршрутов

## Что умеет

```bash
vpn-bypass add ozon.ru
vpn-bypass remove ozon.ru
vpn-bypass list
vpn-bypass sync
vpn-bypass clear
vpn-bypass status
vpn-bypass doctor
vpn-bypass watch --interval 60s
```

## Быстрый запуск

```bash
go build -o vpn-bypass ./cmd/vpn-bypass
sudo ./vpn-bypass add ozon.ru
sudo ./vpn-bypass sync
```

## Конфиг

macOS:

```text
/Library/Application Support/vpn-bypass/config.json
```

Windows:

```text
C:\ProgramData\vpn-bypass\config.json
```

## Важно

Это IP-маршрутизация, а не настоящий domain-based routing.

У сайтов могут быть CDN, дополнительные домены и динамические IP. Поэтому утилита периодически обновляет маршруты.

## Текущий статус

Это первая рабочая версия проекта. Автосервис, Homebrew и winget будут добавлены следующим этапом.
