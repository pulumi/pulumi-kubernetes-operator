FROM pulumi/pulumi:3.177.0-nonroot@sha256:38f182656210ff3c526a33923981d463346e8fb16ba371a16346c7847c625b17

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py