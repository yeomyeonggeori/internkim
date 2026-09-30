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
	&& apt-get install -y --no-install-recommends ca-certificates postgresql-client netcat-openbsd unzip curl jq python3 libfontconfig1 fonts-nanum \
	&& rm -rf /var/lib/apt/lists/*
ARG MOLI_VERSION=1.1.5
ARG AGENT_BROWSER_VERSION=0.32.3
ARG VERSITYGW_VERSION=1.8.0
ARG BUN_VERSION=1.4.2
ARG UV_VERSION=0.11.11
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
		amd64) moliTriple=x86_64-unknown-linux-gnu; moliSHA256=7128ca9b9f7e7bb5ab58b1c6cbf0910a2e22008f4662ea87bd6b8ab8493e3181; agentBrowserAsset=agent-browser-linux-x64; agentBrowserSHA256=243f6e01c4b7dea53ad07d9754df99033c614582d5c685c529a1cb81cafc3ab1; versitygwTarget=x86_64; versitygwSHA256=2ba2c734d10d2c4e651d03182cb4b246656bc735a2f282db7b0b73fba6073467; bunMachine=x64; bunSHA256=36368faef7527875d5ffa52e53cd48021741f2a83eb6208a8dd64068d422a913; uvTriple=x86_64-unknown-linux-gnu; uvSHA256=a767848254391855c96df271e9ca8b7f72dd172d310460447853d25d907b9ae0 ;; \
		arm64) moliTriple=aarch64-unknown-linux-gnu; moliSHA256=76abe24ad32a42e190dd09d3a475f57b91cda66cd8cbf547f602f3aa28963a7b; agentBrowserAsset=agent-browser-linux-arm64; agentBrowserSHA256=87fd2efb67995fc433569f0383260bfee44a785d6d45ca07c77179c45b70de18; versitygwTarget=arm64; versitygwSHA256=b34051d33f5a9c457f790896acb7bd7d7e15ad8d92efb70616b924f37e401910; bunMachine=aarch64; bunSHA256=54328bbc2d9c8e0c9f892c544d66c57a83b84139e34909e5ee81758f1ac8fda7; uvTriple=aarch64-unknown-linux-gnu; uvSHA256=155fe4d3b3cb4bfce118ab4b1380f71515ae874d13d9858171b4f9c26e16684d ;; \
		*) echo "no device browser build for ${TARGETARCH}" >&2; exit 1 ;; \
	esac \
	&& curl -fsSL -o /tmp/moli.tar.gz "https://github.com/lexmount/moli/releases/download/v${MOLI_VERSION}/moli-${moliTriple}.tar.gz" \
	&& echo "${moliSHA256}  /tmp/moli.tar.gz" | sha256sum -c - \
	&& tar -xzf /tmp/moli.tar.gz -C /tmp "moli-v${MOLI_VERSION}-${moliTriple}/moli" \
	&& install -m 0755 "/tmp/moli-v${MOLI_VERSION}-${moliTriple}/moli" /usr/local/bin/moli \
	&& rm -rf /tmp/moli.tar.gz "/tmp/moli-v${MOLI_VERSION}-${moliTriple}" \
	&& curl -fsSL -o /usr/local/bin/agent-browser "https://github.com/vercel-labs/agent-browser/releases/download/v${AGENT_BROWSER_VERSION}/${agentBrowserAsset}" \
	&& echo "${agentBrowserSHA256}  /usr/local/bin/agent-browser" | sha256sum -c - \
	&& chmod 0755 /usr/local/bin/agent-browser \
	&& curl -fsSL -o /tmp/versitygw.tar.gz "https://github.com/versity/versitygw/releases/download/v${VERSITYGW_VERSION}/versitygw_v${VERSITYGW_VERSION}_Linux_${versitygwTarget}.tar.gz" \
	&& echo "${versitygwSHA256}  /tmp/versitygw.tar.gz" | sha256sum -c - \
	&& tar -xzf /tmp/versitygw.tar.gz -C /tmp "versitygw_v${VERSITYGW_VERSION}_Linux_${versitygwTarget}/versitygw" \
	&& install -m 0755 "/tmp/versitygw_v${VERSITYGW_VERSION}_Linux_${versitygwTarget}/versitygw" /usr/local/bin/versitygw \
	&& rm -rf /tmp/versitygw.tar.gz "/tmp/versitygw_v${VERSITYGW_VERSION}_Linux_${versitygwTarget}" \
	&& curl -fsSL -o /tmp/bun.zip "https://github.com/oven-sh/bun/releases/download/bun-v${BUN_VERSION}/bun-linux-${bunMachine}.zip" \
	&& echo "${bunSHA256}  /tmp/bun.zip" | sha256sum -c - \
	&& unzip -q -d /tmp /tmp/bun.zip "bun-linux-${bunMachine}/bun" \
	&& install -m 0755 "/tmp/bun-linux-${bunMachine}/bun" /usr/local/bin/bun \
	&& rm -rf /tmp/bun.zip "/tmp/bun-linux-${bunMachine}" \
	&& curl -fsSL -o /tmp/uv.tar.gz "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/uv-${uvTriple}.tar.gz" \
	&& echo "${uvSHA256}  /tmp/uv.tar.gz" | sha256sum -c - \
	&& tar -xzf /tmp/uv.tar.gz -C /tmp "uv-${uvTriple}/uv" \
	&& install -m 0755 "/tmp/uv-${uvTriple}/uv" /usr/local/bin/uv \
	&& rm -rf /tmp/uv.tar.gz "/tmp/uv-${uvTriple}"

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
COPY assets/document-conversion/requirements.txt /opt/internkim/document-conversion/requirements.txt
COPY host/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY tools/render-company-runtime /usr/local/bin/render-company-runtime
COPY host/runtime.template.json /opt/internkim/runtime.template.json

RUN uv venv --python /usr/bin/python3 /opt/internkim/document-venv \
	&& uv pip install --python /opt/internkim/document-venv/bin/python --requirement /opt/internkim/document-conversion/requirements.txt \
	&& uv cache clean

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
