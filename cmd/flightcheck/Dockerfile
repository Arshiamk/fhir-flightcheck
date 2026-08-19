FROM golang:1.26.6-alpine AS build
WORKDIR /src
COPY go.work ./
COPY services/control-plane/go.mod services/control-plane/go.sum services/control-plane/
COPY cmd/flightcheck/go.mod cmd/flightcheck/
RUN go -C services/control-plane mod download
COPY services/control-plane services/control-plane
COPY cmd/flightcheck cmd/flightcheck
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/flightcheck ./cmd/flightcheck

FROM alpine:3.23
RUN addgroup -S flightcheck && adduser -S -G flightcheck flightcheck
WORKDIR /workspace
COPY --from=build /out/flightcheck /usr/local/bin/flightcheck
USER flightcheck
ENTRYPOINT ["flightcheck"]
