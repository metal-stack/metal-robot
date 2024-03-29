FROM ghcr.io/metal-stack/builder:latest as builder

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /work/bin/metal-robot /metal-robot
CMD ["/metal-robot"]
