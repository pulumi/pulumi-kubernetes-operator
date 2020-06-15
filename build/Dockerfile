# FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
FROM pulumi/pulumi

ENV OPERATOR=/usr/local/bin/pulumi-kubernetes-operator
#    USER_UID=1001 \
#    USER_NAME=pulumi-kubernetes-operator

# install operator binary
COPY build/_output/bin/pulumi-kubernetes-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

RUN useradd -m pulumi-kubernetes-operator
USER pulumi-kubernetes-operator

ENTRYPOINT ["/usr/local/bin/entrypoint"]
