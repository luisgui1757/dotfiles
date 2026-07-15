FROM nixos/nix:2.34.0@sha256:b9c9611c8530fa8049a1215b20638536e1e71dcaf85212e47845112caf3adeea AS nix

FROM ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        bash ca-certificates coreutils curl findutils git gzip passwd procps sudo tar unzip xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -m -s /bin/bash dotfiles \
    && printf '%s\n' 'dotfiles ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/dotfiles \
    && chmod 0440 /etc/sudoers.d/dotfiles \
    && mkdir -p /etc/nix \
    && printf '%s\n' 'experimental-features = nix-command flakes' 'sandbox = false' > /etc/nix/nix.conf

COPY --from=nix --chown=dotfiles:dotfiles /nix /nix
COPY --chown=root:root tests/ci/linux-owner-lifecycle-container.sh /usr/local/bin/dotfiles-linux-owner-lifecycle-container

ENV NIX_REMOTE=local
ENV PATH=/nix/var/nix/profiles/default/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
