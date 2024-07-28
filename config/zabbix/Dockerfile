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
RUN wget "https://dl-cdn.alpinelinux.org/alpine/edge/main/$(apk --print-arch)/libcurl-8.9.0-r0.apk" && apk add libcurl-8.*.apk && rm libcurl-8.*.apk

# restore original user
USER 1997
