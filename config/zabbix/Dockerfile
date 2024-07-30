FROM zabbix/zabbix-server-mysql:latest

LABEL org.opencontainers.image.authors="Dmitry Verkhoturov <paskal.07@gmail.com>" \
      org.opencontainers.image.description="Zabbix Server with limxml2-utils, openssl and coreutils" \
      org.opencontainers.image.source="https://github.com/paskal/terrty.git"

# root to allow installation of the packages
USER 0
# libxml2-utils brings xmllint for sitemap_check.sh
# openssl is required for ssl_cert_check.sh
# coreutils brings date for ssl_cert_check.sh
RUN apk add --update --no-cache sudo libxml2-utils openssl coreutils

# restore original user
USER 1997
