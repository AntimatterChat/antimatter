# First stage - Ubuntu with document processing dependencies and curl for downloading
FROM ubuntu:noble-20251013@sha256:c35e29c9450151419d9448b0fd75374fec4fff364a27f176fb458d472dfc9e54 AS builder
# Setting bash as our shell, and enabling pipefail option
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Build Arguments
ARG PUID=2000
ARG PGID=2000
# AM_PACKAGE is the URL of the Antimatter release archive to install (required), e.g. one built by
# `make package-linux` (dist/antimatter-team-linux-amd64.tar.gz) and served over HTTP.
ARG AM_PACKAGE

# Install needed packages and indirect dependencies
RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
  ca-certificates \
  curl \
  media-types \
  mailcap \
  unrtf \
  wv \
  poppler-utils \
  tidy \
  tzdata \
  && rm -rf /var/lib/apt/lists/*

# Set the antimatter group/user and download Antimatter
RUN test -n "${AM_PACKAGE}" || (echo "the AM_PACKAGE build argument is required" && exit 1) \
  && mkdir -p /antimatter/data /antimatter/plugins /antimatter/client/plugins \
  && groupadd --gid ${PGID} antimatter \
  && useradd --uid ${PUID} --gid ${PGID} --comment "" --home-dir /antimatter antimatter \
  && curl -fL "${AM_PACKAGE}" | tar -xvz \
  && chown -R antimatter:antimatter /antimatter /antimatter/data /antimatter/plugins /antimatter/client/plugins

# Create PostgreSQL client SSL directory structure for ssl_mode=require
RUN mkdir -p /antimatter/.postgresql \
  && chmod 700 /antimatter/.postgresql

# Final stage using distroless for minimal attack surface
FROM gcr.io/distroless/base-debian12

# Some ENV variables
ENV PATH="/antimatter/bin:${PATH}"
ENV MM_SERVICESETTINGS_ENABLELOCALMODE="true"
ENV MM_INSTALL_TYPE="docker"

# Copy over metadata files needed by runtime
COPY --from=builder /etc/mime.types /etc

# Copy CA certificates for SSL/TLS validation with proper ownership
COPY --from=builder --chown=2000:2000 /etc/ssl/certs /etc/ssl/certs

# Copy document processing utilities and necessary support files
COPY --from=builder /usr/bin/pdftotext /usr/bin/pdftotext
COPY --from=builder /usr/bin/wvText /usr/bin/wvText
COPY --from=builder /usr/bin/wvWare /usr/bin/wvWare
COPY --from=builder /usr/bin/unrtf /usr/bin/unrtf
COPY --from=builder /usr/bin/tidy /usr/bin/tidy
COPY --from=builder /usr/share/wv /usr/share/wv

# Copy necessary libraries for document processing utilities
COPY --from=builder /usr/lib/libpoppler.so* /usr/lib/
COPY --from=builder /usr/lib/libfreetype.so* /usr/lib/
COPY --from=builder /usr/lib/libpng.so* /usr/lib/
COPY --from=builder /usr/lib/libwv.so* /usr/lib/
COPY --from=builder /usr/lib/libtidy.so* /usr/lib/
COPY --from=builder /usr/lib/libfontconfig.so* /usr/lib/

# Copy antimatter from builder stage
COPY --from=builder --chown=2000:2000 /antimatter /antimatter

# Copy passwd including the antimatter user
COPY  passwd /etc/passwd

# We should refrain from running as privileged user
USER antimatter

# Healthcheck to make sure container is ready - using amctl instead of curl for distroless compatibility
HEALTHCHECK --interval=30s --timeout=10s \
  CMD ["/antimatter/bin/amctl", "system", "status", "--local"]

# Configure entrypoint and command with proper permissions
WORKDIR /antimatter
CMD ["/antimatter/bin/antimatter"]

EXPOSE 8065 8067 8074

# Declare volumes for mount point directories
VOLUME ["/antimatter/data", "/antimatter/logs", "/antimatter/config", "/antimatter/plugins", "/antimatter/client/plugins"]
