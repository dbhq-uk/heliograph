# =============================================================================
#  ghcr.io/dbhq-uk/heliograph - the CONTROL side, as a container
# =============================================================================
#     docker run -i --rm ghcr.io/dbhq-uk/heliograph mcp     # as an MCP server
#     docker run --rm ghcr.io/dbhq-uk/heliograph estates    # as the CLI
#
#  This is the whole `heliograph` binary, not an MCP-only build. `mcp` is the
#  default command because that is what a directory or an MCP client starts it
#  for, but every other subcommand is here too.
#
#  NOT TO BE CONFUSED WITH ghcr.io/dbhq-uk/heliograph-toolkit, which is the
#  STATION - the far side, built from dbhq-uk/heliograph-skill. The two images
#  sit on opposite sides of the gap and share nothing but the wire format:
#
#    heliograph          your machine. Sends steps, reads logs. This file.
#    heliograph-toolkit  the machine you cannot log into. Runs them.
#
#  -i is not optional for `mcp`: the server speaks JSON-RPC on stdin and
#  stdout, and without an attached stdin it starts, reads EOF and exits, which
#  looks exactly like a crash.
#
#  WHAT THIS IS FOR
#
#  Two things. Directories that verify a server by running it need something
#  they can start, and an OCI image is one of the package types the MCP
#  registry accepts. For ordinary use the released binary is better: it is one
#  static file, and heliograph reads estate configuration from the host's
#  ~/.config, which a container does not have unless it is mounted.
#
#  IT CARRIES NO CREDENTIALS AND NO ESTATE. Started with nothing mounted, the
#  server answers introspection and reports honestly that no estates are
#  configured. That is the correct answer, not a failure: the tools exist, and
#  there is nowhere for them to point yet.
# =============================================================================
FROM golang:1.27-alpine AS build
WORKDIR /src
# The module files first, so a change to the source does not re-resolve
# dependencies. There are two of them and they rarely move.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The version is stamped in, so a directory that starts this image reports a
# real version rather than "dev". Defaulted rather than required, so a plain
# `docker build .` still works.
ARG VERSION=dev
# CGO off: the binary has to run on a distroless image with no libc of its own.
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" -o /heliograph ./cmd/heliograph

# Distroless static: no shell, no package manager, nothing to exec into. The
# server needs none of it, and an MCP server that a directory runs unattended
# should carry the smallest surface it can.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /heliograph /usr/local/bin/heliograph
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/heliograph"]
CMD ["mcp"]

LABEL org.opencontainers.image.title="heliograph" \
      org.opencontainers.image.description="The control side of heliograph: CLI and MCP server. Run commands on a machine you cannot SSH into." \
      org.opencontainers.image.url="https://heliograph.dbhq.uk" \
      org.opencontainers.image.source="https://github.com/dbhq-uk/heliograph" \
      org.opencontainers.image.licenses="MIT"
