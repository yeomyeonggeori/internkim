# The company agent as one image: the binaries entrypoint.sh starts, and nothing
# else. `internkim release host` pushes it; the host installer pulls the tag it
# was stamped with.
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
RUN CGO_ENABLED=0 go build -o /out/blueclaw ./cmd/blueclaw \
	&& CGO_ENABLED=0 go build -o /out/blueclaw-posix-helper ./cmd/blueclaw-posix-helper

FROM oven/bun:1 AS bun-build
WORKDIR /src/blueclaw
COPY .dependency/blueclaw/package.json .dependency/blueclaw/bun.lock ./
COPY .dependency/blueclaw/admin/package.json ./admin/
COPY .dependency/blueclaw/protocol ./protocol
COPY .dependency/blueclaw/chatd ./chatd
RUN bun install --frozen-lockfile --filter ./chatd
WORKDIR /src
COPY host/relay ./relay
RUN cd relay && bun install --frozen-lockfile

FROM debian:bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates postgresql-client netcat-openbsd unzip curl jq python3 python3-venv libfontconfig1 \
	&& rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://bun.sh/install | bash && ln -s /root/.bun/bin/bun /usr/local/bin/bun

ARG MOLI_VERSION=1.1.5
ARG AGENT_BROWSER_VERSION=0.32.3
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
		amd64) moliTriple=x86_64-unknown-linux-gnu; moliSHA256=7128ca9b9f7e7bb5ab58b1c6cbf0910a2e22008f4662ea87bd6b8ab8493e3181; agentBrowserAsset=agent-browser-linux-x64; agentBrowserSHA256=243f6e01c4b7dea53ad07d9754df99033c614582d5c685c529a1cb81cafc3ab1 ;; \
		arm64) moliTriple=aarch64-unknown-linux-gnu; moliSHA256=76abe24ad32a42e190dd09d3a475f57b91cda66cd8cbf547f602f3aa28963a7b; agentBrowserAsset=agent-browser-linux-arm64; agentBrowserSHA256=87fd2efb67995fc433569f0383260bfee44a785d6d45ca07c77179c45b70de18 ;; \
		*) echo "no device browser build for ${TARGETARCH}" >&2; exit 1 ;; \
	esac \
	&& curl -fsSL -o /tmp/moli.tar.gz "https://github.com/lexmount/moli/releases/download/v${MOLI_VERSION}/moli-${moliTriple}.tar.gz" \
	&& echo "${moliSHA256}  /tmp/moli.tar.gz" | sha256sum -c - \
	&& tar -xzf /tmp/moli.tar.gz -C /tmp "moli-v${MOLI_VERSION}-${moliTriple}/moli" \
	&& install -m 0755 "/tmp/moli-v${MOLI_VERSION}-${moliTriple}/moli" /usr/local/bin/moli \
	&& rm -rf /tmp/moli.tar.gz "/tmp/moli-v${MOLI_VERSION}-${moliTriple}" \
	&& curl -fsSL -o /usr/local/bin/agent-browser "https://github.com/vercel-labs/agent-browser/releases/download/v${AGENT_BROWSER_VERSION}/${agentBrowserAsset}" \
	&& echo "${agentBrowserSHA256}  /usr/local/bin/agent-browser" | sha256sum -c - \
	&& chmod 0755 /usr/local/bin/agent-browser

COPY --from=go-build /out/blueclaw /opt/blueclaw/bin/blueclaw
COPY --from=go-build /out/blueclaw-posix-helper /usr/local/bin/blueclaw-posix-helper
COPY --from=go-build /out/internkim-capabilityd /usr/local/bin/internkim-capabilityd
COPY --from=go-build /out/internkim-admind /usr/local/bin/internkim-admind
COPY --from=go-build /out/internkim-maild /usr/local/bin/internkim-maild
COPY --from=bun-build /src/blueclaw /opt/blueclaw-workspace
COPY --from=bun-build /src/relay /opt/internkim/host/relay
COPY web/src/lib/person-name.ts /opt/internkim/web/src/lib/person-name.ts
COPY .dependency/blueclaw/migrations /opt/blueclaw/migrations
COPY .dependency/internkim-plugin/skills /opt/internkim/skills
COPY host/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY tools/render-company-runtime /usr/local/bin/render-company-runtime
COPY host/runtime.template.json /opt/internkim/runtime.template.json

# entrypoint.sh calls these by name; a wrapper keeps the script free of paths.
RUN printf '#!/bin/sh\nexec bun run /opt/blueclaw-workspace/chatd/src/main.ts "$@"\n' > /usr/local/bin/chatd \
	&& printf '#!/bin/sh\nexec bun run /opt/internkim/host/relay/relay.ts "$@"\n' > /usr/local/bin/internkim-relay \
	&& printf '#!/bin/sh\ncd /workspace\nexec setpriv --reuid blueclaw --regid blueclaw --init-groups env HOME=/home/blueclaw /opt/blueclaw/bin/blueclaw "$@"\n' > /usr/local/bin/blueclaw \
	&& chmod 0755 /usr/local/bin/chatd /usr/local/bin/internkim-relay /usr/local/bin/blueclaw /usr/local/bin/entrypoint.sh \
	&& mkdir -p /run/internkim /workspace /etc/blueclaw

RUN groupadd --system blueclaw \
	&& useradd --system --gid blueclaw --create-home --home-dir /home/blueclaw --shell /usr/sbin/nologin blueclaw \
	&& setpriv --reuid blueclaw --regid blueclaw --init-groups id -un | grep -qx blueclaw \
	&& chown root:root /usr/local/bin/blueclaw-posix-helper \
	&& chmod 4755 /usr/local/bin/blueclaw-posix-helper \
	&& /usr/local/bin/blueclaw-posix-helper capabilities \
		| jq -e '.version >= 2 and (.capabilities | index("exec")) and (.capabilities | index("fs"))' >/dev/null

RUN /usr/local/bin/entrypoint.sh --check-programs

CMD ["/usr/local/bin/entrypoint.sh"]
