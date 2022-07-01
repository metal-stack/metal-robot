FROM metalstack/builder:latest as builder

FROM alpine:3.16
RUN apk add --no-cache tini ca-certificates
COPY --from=builder /work/bin/metal-robot /metal-robot
CMD ["/metal-robot"]
