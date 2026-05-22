# syntax=docker/dockerfile:1.7
#
# Release image. The workflow (.github/workflows/container.yml) builds the
# linux/amd64 + linux/arm64 binaries into dist/linux_<arch>/huetension and
# then runs `docker buildx build` with platforms set to both — buildx
# expands TARGETARCH per platform so the right binary lands in each
# image variant.
#
# For source builds during development use Dockerfile.dev instead.

FROM alpine:3.23
ARG TARGETARCH
COPY dist/linux_${TARGETARCH}/huetension /usr/local/bin/huetension
RUN addgroup -S huetension && adduser -S -G huetension huetension
USER huetension
EXPOSE 8080 7337
ENTRYPOINT ["/usr/local/bin/huetension"]
CMD ["web", "--address", "0.0.0.0:8080"]
