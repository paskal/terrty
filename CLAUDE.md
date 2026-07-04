## Project Overview
- Infrastructure/server configuration repo for terrty.net and related domains
- Docker Compose based deployment on Oracle Linux (ARM64)

## Deployment
- Server hostname: `terrty` (accessible via SSH)
- Sync files with: `rsync -avz <file> terrty:~/terrty/<path>`
- Apply changes: `ssh terrty "cd ~/terrty && docker compose up -d"`
- All services defined in `docker-compose.yml`

## Gotchas
- **`zabbix-web` needs `DB_SERVER_PORT=3306` even though it connects via the MySQL socket.**
  Zabbix 7.4.11 changed the frontend config template from `getenv('DB_SERVER_PORT')` (returns
  `false` when unset, coerced to `int 0`) to `env_string('DB_SERVER_PORT')` (returns `''` when
  unset). Under PHP 8 an empty string is invalid for `mysqli::real_connect()`'s `?int $port`
  argument, so an unset port throws a fatal `TypeError` and every frontend page returns HTTP 500.
  Setting a numeric port sidesteps it; the value is ignored because the connection uses the socket.
  This is applied on the server via an untracked `docker-compose.override.yml`, so do not delete
  that file.
  - Fixed upstream in zabbix-docker `cc028ed` (2026-07-01, branches `master` + `7.4`), which
    replaces `env_string` with an `env_int` helper.
  - **Waiting for a fixed published image:** as of 2026-07-05 no stable image contains the fix.
    The newest stable `zabbix/zabbix-web-nginx-mysql` tags (`latest`, `7.4.11`) predate it
    (2026-06-21 and 2026-06-03); only the `trunk-*` nightlies carry it. Once a stable `7.4.x`
    image published after 2026-07-01 appears, drop `DB_SERVER_PORT` and this override.
