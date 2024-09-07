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

### Zabbix Agent

Zabbix Agent setup in
