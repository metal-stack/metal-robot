FROM gcr.io/distroless/static-debian13:nonroot
COPY bin/metal-robot /metal-robot
CMD ["/metal-robot"]
