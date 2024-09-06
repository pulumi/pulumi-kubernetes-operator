ARG BASE_IMAGE=pulumi/pulumi:latest
FROM ${BASE_IMAGE}
ARG UID=1000
ARG GID=1000

RUN addgroup --gid $GID pulumi && \
    adduser --uid $UID --gid $GID --disabled-password --gecos "" pulumi && \
    echo 'pulumi ALL=(ALL) NOPASSWD: ALL' >> /etc/sudoers
USER pulumi:pulumi