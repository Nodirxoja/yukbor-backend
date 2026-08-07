# One multi-stage Dockerfile for all services; pick the binary with SERVICE.
FROM golang:1.26-alpine AS build
ARG SERVICE
WORKDIR /src
# Dependency layer: only invalidated when go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/${SERVICE}

FROM alpine:3.20
# busybox wget backs the compose healthchecks; ca-certificates for future
# outbound calls to MyID / SMS gateways.
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/app /bin/app
ENTRYPOINT ["/bin/app"]
