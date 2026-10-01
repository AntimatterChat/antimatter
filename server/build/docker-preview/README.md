# Antimatter Docker Preview Image

This is a Docker image to install Antimatter in *Preview Mode* for exploring product functionality on a single machine using Docker.

Note: This configuration should not be used in production, as it’s using a known password string and contains other non-production configuration settings, and it does not support upgrade. For a production installation with Docker, use the image built by [antimatter-docker](https://github.com/AntimatterChat/antimatter-docker).

## Usage

Build the image from an Antimatter release archive (`make package-linux` in `server/` writes `dist/antimatter-team-linux-amd64.tar.gz`), served at a URL Docker can fetch:

```
docker build --build-arg AM_PACKAGE=<url of antimatter-team-linux-amd64.tar.gz> -t antimatter-preview .
docker run --name antimatter-preview -d --publish 8065:8065 --add-host dockerhost:127.0.0.1 antimatter-preview
```
