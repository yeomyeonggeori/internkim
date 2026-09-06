# The company agent as one image: the binaries entrypoint.sh starts, and nothing
# else. Building here rather than shipping a tarball means a clone of this
# repository is enough to run the thing it describes.
FROM golang:1.26-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
COPY .dependency/blueclaw/go.mod .dependency/blueclaw/go.sum ./.dependency/blueclaw/
COPY .dependency/blueclaw/.dependency/bluecollar/go.mod .dependency/blueclaw/.dependency/bluecollar/go.sum ./.dependency/blueclaw/.dependency/bluecollar/
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/internkim-capabilityd ./cmd/internkim-capabilityd \
	&& CGO_ENABLED=0 go build -o /out/internkim-admind ./cmd/internkim-admind \
	&& CGO_ENABLED=0 go build -o /out/internkim-maild ./cmd/internkim-maild \
	&& CGO_ENABLED=0 go build -o /out/internkim-host-identity ./cmd/internkim-host-identity
WORKDIR /src/.dependency/blueclaw
RUN CGO_ENABLED=0 go build -o /out/blueclaw ./cmd/blueclaw \
	&& CGO_ENABLED=0 go build -o /out/blueclaw-posix-helper ./cmd/blueclaw-posix-helper

FROM oven/bun:1 AS bun-build
WORKDIR /src/blueclaw
COPY .dependency/blueclaw/package.json .dependency/blueclaw/bun.lock ./
COPY .dependency/blueclaw/protocol/package.json ./protocol/package.json
COPY .dependency/blueclaw/chatd/package.json ./chatd/package.json
COPY .dependency/blueclaw/admin/package.json ./admin/package.json
RUN bun install --frozen-lockfile
COPY .dependency/blueclaw/protocol ./protocol
COPY .dependency/blueclaw/chatd ./chatd
COPY .dependency/blueclaw/admin ./admin
COPY host/relay /src/host/relay
COPY web/src/lib/person-name.ts /src/web/src/lib/person-name.ts
COPY web/src/lib/i18n/locale.ts /src/web/src/lib/i18n/locale.ts
RUN cd /src/host/relay \
	&& bun install --frozen-lockfile \
	&& bun build --compile --outfile /out/internkim-relay relay.ts
RUN bun build --compile --outfile /out/chatd chatd/src/main.ts

FROM debian:bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates postgresql-client netcat-openbsd unzip curl jq python3 python3-venv acl \
	&& rm -rf /var/lib/apt/lists/*
RUN groupadd --system blueclaw \
	&& useradd --system --gid blueclaw --create-home --home-dir /var/lib/blueclaw --shell /bin/sh blueclaw
COPY --from=bun-build /usr/local/bin/bun /usr/local/bin/bun

COPY --from=go-build /out/blueclaw /usr/local/bin/blueclaw
COPY --from=go-build /out/blueclaw-posix-helper /usr/local/bin/blueclaw-posix-helper
RUN chown root:root /usr/local/bin/blueclaw-posix-helper \
	&& chmod 4755 /usr/local/bin/blueclaw-posix-helper
COPY --from=go-build /out/internkim-capabilityd /usr/local/bin/internkim-capabilityd
COPY --from=go-build /out/internkim-admind /usr/local/bin/internkim-admind
COPY --from=go-build /out/internkim-maild /usr/local/bin/internkim-maild
COPY --from=go-build /out/internkim-host-identity /usr/local/bin/internkim-host-identity
COPY --from=bun-build /out/chatd /usr/local/bin/chatd
COPY --from=bun-build /out/internkim-relay /usr/local/bin/internkim-relay
COPY .dependency/blueclaw/migrations /opt/blueclaw/migrations
COPY .dependency/internkim-plugin/skills /opt/internkim/skills
COPY host/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY tools/render-company-runtime /usr/local/bin/render-company-runtime
COPY host/runtime.template.json /opt/internkim/runtime.template.json

# entrypoint.sh calls these by name.
RUN chmod 0755 /usr/local/bin/chatd /usr/local/bin/internkim-relay /usr/local/bin/entrypoint.sh \
	&& mkdir -p /run/internkim /workspace /etc/blueclaw

RUN /usr/local/bin/entrypoint.sh --check-programs

WORKDIR /workspace
CMD ["/usr/local/bin/entrypoint.sh"]
