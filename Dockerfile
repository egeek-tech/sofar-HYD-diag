# Runtime-only image. Binary is cross-compiled by the release workflow
# and dropped into dist/ before docker build runs.
# For local builds: `make docker` produces dist/server-amd64 first.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETARCH
COPY dist/server-${TARGETARCH} /server
EXPOSE 8080
ENTRYPOINT ["/server"]
