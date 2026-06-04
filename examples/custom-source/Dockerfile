FROM pulumi/pulumi:3.245.0-nonroot@sha256:7cd613954962e8597aa6f66e5d22f0a6099e2b2dc1bc295ebc2add2595309b93

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py