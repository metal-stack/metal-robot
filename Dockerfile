FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY bin/metal-robot /metal-robot
CMD ["/metal-robot"]
