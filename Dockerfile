FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -o /out/bandwidth-exporter ./cmd/exporter \
    && mkdir -p /out/plugins

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /out/bandwidth-exporter .
COPY --from=build /out/plugins ./plugins
EXPOSE 9598
ENTRYPOINT ["./bandwidth-exporter"]
