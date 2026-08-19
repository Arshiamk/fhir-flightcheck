FROM golang:1.26.6-alpine AS build
WORKDIR /src
COPY go.work ./
COPY services/control-plane/go.mod services/control-plane/go.sum services/control-plane/
COPY cmd/flightcheck/go.mod cmd/flightcheck/
RUN go -C services/control-plane mod download
COPY services/control-plane services/control-plane
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/control-plane ./services/control-plane/cmd/control-plane

FROM alpine:3.23
RUN addgroup -S flightcheck && adduser -S -G flightcheck flightcheck
WORKDIR /app
COPY --from=build /out/control-plane /usr/local/bin/control-plane
COPY packages/rule-packs /app/packages/rule-packs
USER flightcheck
EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O - http://127.0.0.1:8081/healthz >/dev/null || exit 1
ENTRYPOINT ["control-plane"]
