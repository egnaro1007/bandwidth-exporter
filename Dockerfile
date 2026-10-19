FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -o /out/bandwidth-exporter ./cmd/exporter \
    && mkdir -p /out/plugins \
    && go build -buildmode=plugin -o /out/plugins/3sg.so ./plugins/3sg \
    && go build -buildmode=plugin -o /out/plugins/megamax.so ./plugins/megamax

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /out/bandwidth-exporter .
COPY --from=build /out/plugins ./plugins
EXPOSE 9598
ENTRYPOINT ["./bandwidth-exporter"]
