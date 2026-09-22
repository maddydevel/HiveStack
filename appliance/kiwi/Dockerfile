# HiveStack SLES 15 SP7 Appliance Builder
#
# This Dockerfile creates a SLES 15 SP7 based builder environment
# with kiwi-ng for building the HiveStack appliance ISO.
#
# Build:
#   docker build -t hivestack-builder -f appliance/kiwi/Dockerfile .
#
# Run:
#   docker run --privileged \
#     -v /home/maddy/HiveStack:/project \
#     -v /tmp/kiwi-output:/output \
#     hivestack-builder
#
# The ISO will be output to /tmp/kiwi-output/

FROM registry.suse.com/suse/sle15:15.7

# Install kiwi-ng and dependencies
RUN zypper --non-interactive refresh && \
    zypper --non-interactive install -y \
        kiwi-ng \
        python3-kiwi \
        xorriso \
        grub2 \
        grub2-x86_64-efi \
        shim \
        systemd \
        && zypper clean -a

# Set working directory
WORKDIR /workspace

# Copy build script
COPY build.sh /workspace/build.sh
RUN chmod +x /workspace/build.sh

# Default command
CMD ["/workspace/build.sh"]
