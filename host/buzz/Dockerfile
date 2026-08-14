FROM debian:bookworm-slim

# The relay's push delivery worker builds an HTTPS client at startup and panics
# on an image with no trust store, which a slim base does not carry.
RUN apt-get update \
	&& apt-get install --no-install-recommends --yes ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/opt/buzz/buzz-relay"]
