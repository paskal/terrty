[![Build Status](https://github.com/paskal/terrty/workflows/build/badge.svg)](https://github.com/paskal/terrty/actions/workflows/ci-build.yml) [![Pull Status](https://github.com/paskal/terrty/workflows/pull/badge.svg)](https://github.com/paskal/terrty/actions/workflows/ci-pull.yml)

Source code for different (mostly, monitoring) services running on <https://terrty.net> which is primarily my blog.

Contains Zabbix Server [![Image Size](https://img.shields.io/docker/image-size/paskal/zabbix-server-mysql)](https://hub.docker.com/r/paskal/zabbix-server-mysql), Plausible, Remark42, Adminer, Newscope, Lighthouse CI, a small answers store for private pages, and Nginx serving https://terrty.net and https://ksinia.net, together with their databases (MySQL, PostgreSQL, ClickHouse).

For IPv6 work on Oracle Linux inside Oracle Cloud run:

```shell
sudo firewall-cmd --zone=public --add-port=80/tcp --permanent
sudo firewall-cmd --zone=public --add-port=443/tcp --permanent
sudo firewall-cmd --zone=public --add-port=443/udp --permanent
sudo firewall-cmd --reload
```

The Zabbix server port 10051 stays out of the public zone: Docker publishes it past firewalld, so agent addresses are allow-listed in the `DOCKER-USER` chain with firewalld direct rules (`sudo firewall-cmd --permanent --direct --get-all-rules`).

### Container Monitoring with Telegram Alerts

The `telegram-notifier` service monitors Docker containers and sends Telegram alerts when they stop, restart, or become unhealthy. Currently configured to monitor `zabbix-server`, `zabbix-web`, and `lhci` containers.

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
   docker compose up -d telegram-notifier
   ```

**To monitor additional containers:** Add the label `telegram-notifier.monitor: true` to any service in `docker-compose.yml`.

### Docker disk cleanup

`config/cron/docker-prune.cron` removes dangling images and week-old build cache every night. Install it after cloning, and again after editing it:

```shell
sudo install -o root -g root -m 0644 config/cron/docker-prune.cron /etc/cron.d/docker-prune
```

### Answers on private pages

`rodnya-api` (`config/rodnya-api`) stores what visitors select and write on the private static pages under `/rodnya/<16 hex>/`; nginx sends `/rodnya/<page>/api/` to it. It accepts a page only if `private/rodnya/<page>/index.html` exists, keeps one append-only log per page in `private/rodnya-data/<page>.jsonl`, and shows all answers at `/rodnya/<page>/api/admin/<ADMIN_KEY>/` with `export.json` and `log.jsonl` downloads there. Setup after cloning:

```shell
openssl rand -hex 16 | sed 's/^/ADMIN_KEY=/' > private/environment/rodnya-api.env
mkdir -p private/rodnya-data
docker run --rm -v "$PWD/private/rodnya-data:/d" alpine chown 65534:65534 /d
docker compose up -d --build rodnya-api
```

The image is built on the server, so rerun the last command after changing its code.

### Zabbix Agent

Zabbix Agent setup for monitored hosts is in the separate file, `docker-compose-zabbix-agent.yml`.
