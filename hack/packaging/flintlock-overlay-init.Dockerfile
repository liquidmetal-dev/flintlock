# The OCI image that carries the overlay initrd. The build context is the
# directory that holds flintlock-overlay-initrd_<arch>.img for every
# platform of the image, as `make build-release-overlay-initrd` writes it.
# flintlock mounts the image and passes initrd.img to the VMM.
FROM scratch
ARG TARGETARCH
COPY flintlock-overlay-initrd_${TARGETARCH}.img /initrd.img
