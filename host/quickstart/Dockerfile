ARG BUZZ_IMAGE
ARG HOST_IMAGE
FROM ${BUZZ_IMAGE} AS buzz
FROM ${HOST_IMAGE}
COPY --from=buzz /usr/local/bin/buzz-admin /usr/local/bin/buzz-admin
