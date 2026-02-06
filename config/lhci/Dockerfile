FROM node:18-bullseye-slim

LABEL org.opencontainers.image.authors="Dmitry Verkhoturov <paskal.07@gmail.com>" \
      org.opencontainers.image.description="Lighthouse CI Server" \
      org.opencontainers.image.source="https://github.com/paskal/terrty.git"

RUN apt-get update && apt-get install -y python3 build-essential && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/src/lhci
RUN npm install @lhci/cli@0.15.x @lhci/server@0.15.x sqlite3@^5.0.0

VOLUME /data
EXPOSE 9001

ENTRYPOINT ["node_modules/.bin/lhci", "server"]
CMD ["--port=9001", "--storage.storageMethod=sql", "--storage.sqlDialect=sqlite", "--storage.sqlDatabasePath=/data/lhci.db"]
