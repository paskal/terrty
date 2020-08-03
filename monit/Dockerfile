FROM alpine:latest

LABEL maintainer="Dmitry Verkhoturov"

# monit environment variables
ARG MONIT2TELEGRAM_URL=https://github.com/matriphe/monit2telegram.git

ADD monit2telegram.sh /bin/monit2telegram

# Compile and install monit + monit2telegram
RUN \
    echo "*** install monit and dependencies ***" && \
    apk add --update --no-cache monit bash curl shadow && \
    echo "*** install monit2telegram ***" && \
    wget https://raw.githubusercontent.com/matriphe/monit2telegram/master/sendtelegram.sh -O /bin/sendtelegram && \
    chmod +x /bin/sendtelegram && \
    chmod +x /bin/monit2telegram

RUN \
    echo "**** create monit user and make our folders ****" && \
    groupmod -g 1000 users && \
    useradd -u 1000 -U -d /config -s /bin/false monit && \
    usermod -G users monit && \
    mkdir /config && \
    chown monit:users /config

EXPOSE 2812

CMD ["monit", "-I", "-B"]
