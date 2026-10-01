# Copyright (c) 2016-present Mattermost, Inc. All Rights Reserved.
# See License.txt for license information.
FROM postgres:14

RUN apt-get update && apt-get install -y ca-certificates

# URL of the Antimatter release archive to install (required), e.g. one built by
# `make package-linux` (dist/antimatter-team-linux-amd64.tar.gz) and served over HTTP.
ARG AM_PACKAGE
#
# Configure PostgreSQL
#

ENV POSTGRES_USER=mmuser
ENV POSTGRES_PASSWORD=mostest_password
ENV POSTGRES_DB=mattermost_test

#
# Configure Antimatter
#
WORKDIR /mm

# Copy over files
RUN test -n "${AM_PACKAGE}" || (echo "the AM_PACKAGE build argument is required" && exit 1)
ADD ${AM_PACKAGE} ./antimatter.tar.gz
RUN tar -zxvf antimatter.tar.gz && rm antimatter.tar.gz
ADD config_docker.json ./antimatter/config/config_docker.json
ADD docker-entry.sh .

RUN chmod +x ./docker-entry.sh
ENTRYPOINT ./docker-entry.sh

# Antimatter environment variables
ENV PATH="/mm/antimatter/bin:${PATH}"

# Create default storage directory
RUN mkdir ./antimatter-data
VOLUME /mm/antimatter-data

# Ports
EXPOSE 8065
