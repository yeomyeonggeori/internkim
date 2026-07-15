FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      postgresql-client \
      netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

COPY bin/internkim-capabilityd /usr/local/bin/internkim-capabilityd
COPY bin/blueclaw /usr/local/bin/blueclaw
COPY bin/blueclaw-posix-helper /usr/local/bin/blueclaw-posix-helper
COPY bin/internkim-admind /usr/local/bin/internkim-admind
RUN chmod 0755 /usr/local/bin/internkim-capabilityd /usr/local/bin/blueclaw /usr/local/bin/internkim-admind \
    && chmod 4755 /usr/local/bin/blueclaw-posix-helper

COPY migrations /opt/blueclaw/migrations
COPY board-ui /opt/internkim/board-ui
COPY mattermost-plugins /opt/internkim/mattermost-plugins
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh \
    && mkdir -p /run/internkim /workspace /etc/blueclaw /secrets /workspace/.admind/state \
       /root/.internkim/secrets /root/.internkim/state/admin /root/.internkim/env

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
