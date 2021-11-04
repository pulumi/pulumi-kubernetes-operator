FROM pulumi/pulumi:3.17.0

ENV OPERATOR=/usr/local/bin/pulumi-kubernetes-operator

# install operator binary
COPY pulumi-kubernetes-operator ${OPERATOR}

COPY build/bin/* /usr/local/bin/
RUN  /usr/local/bin/user_setup

RUN useradd -m pulumi-kubernetes-operator
RUN mkdir -p /home/pulumi-kubernetes-operator/.ssh \
    && touch /home/pulumi-kubernetes-operator/.ssh/known_hosts \
    && chmod 700 /home/pulumi-kubernetes-operator/.ssh \
    && chown -R pulumi-kubernetes-operator:pulumi-kubernetes-operator /home/pulumi-kubernetes-operator/.ssh

USER pulumi-kubernetes-operator

ENV XDG_CONFIG_HOME=/tmp/.config
ENV XDG_CACHE_HOME=/tmp/.cache
ENV XDG_CONFIG_CACHE=/tmp/.cache
ENV GOCACHE=/tmp/.cache/go-build
ENV GOPATH=/tmp/.cache/go

ENTRYPOINT ["/usr/local/bin/entrypoint"]
