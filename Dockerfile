FROM golang:1.22.5 AS builder
COPY . /app
WORKDIR /app

ARG VERSION="unknown"
ARG BUILD_DATE=""
ARG HASH="xxxxxxxx"
ARG FLAGS="-X 'main.Version=${VERSION}' -X 'main.Hash=${HASH}' -X 'main.BuildDate=${BUILD_DATE}'"
RUN CGO_ENABLED=0 go build -mod=vendor -ldflags="$FLAGS"

FROM scratch

# next string prevents: 'x509: certificate signed by unknown authority' error, do not remove!
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/nats-configurator ./nats-configurator
ENTRYPOINT ["./nats-configurator"]