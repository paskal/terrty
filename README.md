[![Build Status](https://github.com/paskal/terrty/workflows/build/badge.svg)](https://github.com/paskal/terrty/actions/workflows/ci-build.yml) [![Pull Status](https://github.com/paskal/terrty/workflows/pull/badge.svg)](https://github.com/paskal/terrty/actions/workflows/ci-pull.yml)

Source code for different (mostly, monitoring) services running on <https://terrty.net> which is primarily my blog.

Contains Zabbix Server [![Image Size](https://img.shields.io/docker/image-size/paskal/zabbix-server-mysql)](https://hub.docker.com/r/paskal/zabbix-server-mysql), Plausible, Remark42, Adminer, Grafana, Nginx for https://terrty.net and https://ksinia.net, and databases (MySQL, PostgreSQL ClickHouse, GeoIP) for these.

For IPv6 work on Oracle Linux inside Oracle Cloud run:

```shell
sudo firewall-cmd --zone=public --add-port=80/tcp --permanent
sudo firewall-cmd --zone=public --add-port=443/tcp --permanent
sudo firewall-cmd --zone=public --add-port=443/udp --permanent
sudo firewall-cmd --zone=public --add-port=10051/tcp --permanent
sudo firewall-cmd --zone=public --add-port=51820/udp --permanent
sudo firewall-cmd --zone=public --add-port=51821/tcp --permanent
sudo firewall-cmd --reload
```

### VPN Servers

Example of VPN servers setup is in the separate file, `docker-compose-vpn.yml`.

### Container Monitoring with Telegram Alerts

The `telegram-notifier` service monitors Docker containers and sends Telegram alerts when they stop, restart, or become unhealthy. Currently configured to monitor `zabbix-server` and `zabbix-web` containers.

**Setup:**

1. Create a Telegram bot:
   - Message [@BotFather](https://t.me/BotFather) on Telegram
   - Send `/newbot` and follow the instructions
   - Copy the bot token

2. Get your chat ID:
   - Message [@userinfobot](https://t.me/userinfobot) on Telegram
   - Copy your chat ID

3. Update `private/environment/telegram-notifier.env`:
   - Set `TELEGRAM_NOTIFIER_BOT_TOKEN` to your bot token
   - Set `TELEGRAM_NOTIFIER_CHAT_ID` to your chat ID

4. Start the monitoring service:
   ```shell
   docker-compose up -d telegram-notifier
   ```

**To monitor additional containers:** Add the label `telegram-notifier.monitor: true` to any service in `docker-compose.yml`.

### Zabbix Agent

Zabbix Agent setup in
