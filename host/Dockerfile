# The company agent as one image: the binaries entrypoint.sh starts, and nothing
# else. Building here rather than shipping a tarball means a clone of this
# repository is enough to run the thing it describes.
FROM golang:1.26-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
COPY .dependency/blueclaw/go.mod .dependency/blueclaw/go.sum ./.dependency/blueclaw/
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/internkim-capabilityd ./cmd/internkim-capabilityd \
	&& CGO_ENABLED=0 go build -o /out/internkim-admind ./cmd/internkim-admind \
	&& CGO_ENABLED=0 go build -o /out/internkim-maild ./cmd/internkim-maild
WORKDIR /src/.dependency/blueclaw
RUN CGO_ENABLED=0 go build -o /out/blueclaw ./cmd/blueclaw

FROM oven/bun:1 AS bun-build
WORKDIR /src
COPY .dependency/blueclaw/chatd ./chatd
RUN cd chatd && bun install --frozen-lockfile
COPY host/relay ./relay
RUN cd relay && bun install --frozen-lockfile

FROM debian:bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates postgresql-client netcat-openbsd unzip curl jq \
	&& rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://bun.sh/install | bash && ln -s /root/.bun/bin/bun /usr/local/bin/bun

COPY --from=go-build /out/blueclaw /usr/local/bin/blueclaw
COPY --from=go-build /out/internkim-capabilityd /usr/local/bin/internkim-capabilityd
COPY --from=go-build /out/internkim-admind /usr/local/bin/internkim-admind
COPY --from=go-build /out/internkim-maild /usr/local/bin/internkim-maild
COPY --from=bun-build /src/chatd /opt/chatd
COPY --from=bun-build /src/relay /opt/relay
COPY .dependency/blueclaw/migrations /opt/blueclaw/migrations
COPY host/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY tools/render-company-runtime /usr/local/bin/render-company-runtime
COPY host/runtime.template.json /opt/internkim/runtime.template.json

# entrypoint.sh calls these by name; a wrapper keeps the script free of paths.
RUN printf '#!/bin/sh\nexec bun run /opt/chatd/src/main.ts "$@"\n' > /usr/local/bin/chatd \
	&& printf '#!/bin/sh\nexec bun run /opt/relay/relay.ts "$@"\n' > /usr/local/bin/internkim-relay \
	&& chmod 0755 /usr/local/bin/chatd /usr/local/bin/internkim-relay /usr/local/bin/entrypoint.sh \
	&& mkdir -p /run/internkim /workspace /etc/blueclaw

CMD ["/usr/local/bin/entrypoint.sh"]
