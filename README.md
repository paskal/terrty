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

The server accepts only PSK-encrypted connections, from agents and from `zabbix_sender` alike, and each host has its own key. To add a host:

1. Allow-list its public address for port 10051 (see the firewall note above):
   ```shell
   for m in "" --permanent; do sudo firewall-cmd $m --direct --add-rule ipv4 filter DOCKER-USER 0 -i <public-interface> -p tcp --dport 10051 -s <host-ip> -j RETURN; done
   ```
2. On the monitored host, next to `docker-compose-zabbix-agent.yml`, create the key readable by root only:
   ```shell
   (umask 077; openssl rand -hex 32 > zabbix-agent.psk)
   ```
   An agent that runs as its own user (uid 1997) instead of root needs `chown 1997:1997 zabbix-agent.psk`.
3. In Zabbix, create the host with `ZBX_HOSTNAME` as its name. On its Encryption tab set "Connections to host" to PSK, tick only PSK under "Connections from host", and enter `ZBX_TLSPSKIDENTITY` and the key. An identity can carry only one key across the whole server, so give every host its own.
4. Start the agent: `docker compose -f docker-compose-zabbix-agent.yml up -d`. Within two minutes the host shows the agent as available, and the server log has no `connection of type "unencrypted" is not allowed` lines for it.

`zabbix_sender` on such a host runs inside the agent container with the same key, so its values land on that host:

```shell
docker exec zabbix-agent sh -c 'exec zabbix_sender -z "$ZBX_SERVER_HOST" -s "$ZBX_HOSTNAME" --tls-connect psk --tls-psk-identity "$ZBX_TLSPSKIDENTITY" --tls-psk-file "$ZBX_TLSPSKFILE" "$@"' zabbix_sender -k <key> -o <value>
```

The agent on this server itself (host "Zabbix server") reads its key from `private/environment/zabbix-agent.psk`, created the same way; compose refuses to start the agent without it.
