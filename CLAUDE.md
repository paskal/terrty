## Project Overview
- Infrastructure/server configuration repo for terrty.net and related domains
- Docker Compose based deployment on Oracle Linux (ARM64)

## Deployment
- Server hostname: `terrty` (accessible via SSH)
- Sync files with: `rsync -avz <file> terrty:~/terrty/<path>`
- Apply changes: `ssh terrty "cd ~/terrty && docker compose up -d"`
- All services defined in `docker-compose.yml`
