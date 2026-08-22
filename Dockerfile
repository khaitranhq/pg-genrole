# syntax=docker/dockerfile:1
# pg-genrole: periodic role/permission refresh against PostgreSQL
# Build: docker build -f docker/Dockerfile -t pg-genrole .

# ---- build ----
# golang:1.26.5-alpine
FROM golang@sha256:111d79159b2326f7e80c4a4706e1ba166acb0e2611df853955f3621828cd49e8 AS build

WORKDIR /src

COPY go.mod go.sum ./

# Cache modules and build artifacts across builds
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pg-genrole ./cmd/pg-genrole

# ---- runtime ----
# dhi.io/golang:1.26-alpine
FROM dhi.io/golang@sha256:53529b76cd901b3a3b8d40e2c685f5c100e2b10d3109d879bafbf0de74699522

COPY --chmod=755 --from=build /out/pg-genrole /usr/local/bin/pg-genrole

ENTRYPOINT ["/usr/local/bin/pg-genrole"]
