FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/sota-headless ./cmd/sota-headless

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/sota-headless /usr/local/bin/sota-headless
RUN mkdir -p /app/bin /app/runtime /app/state /app/rule_sets
EXPOSE 16698 2080
ENTRYPOINT ["sota-headless"]
