FROM ghcr.io/metal-stack/builder:latest AS builder

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=builder /work/bin/metal-robot /metal-robot
CMD ["/metal-robot"]
