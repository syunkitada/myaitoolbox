FROM rockylinux:9

WORKDIR /root

ENV HOME=/root

RUN dnf install -y epel-release && \
    dnf install -y \
      systemd sudo man vim findutils procps-ng \
      git wget make diffutils \
      traceroute iproute \
      python3.12 python3.12-devel python3.12-pip

# Install Node
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.5/install.sh | bash && \
    \. "$HOME/.nvm/nvm.sh" && \
    nvm install 24 && \
    npm install -g opencode-ai

# Install DockerCompose
RUN dnf -y install dnf-plugins-core && \
    dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo && \
    dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

RUN dnf clean all && rm -rf /var/cache/dnf/*

# Remove unused systemd services
RUN ls /lib/systemd/system/*.wants/* | grep -v journald | xargs rm -rf

ENV PATH=$HOME/.local/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

CMD ["/usr/sbin/init"]
