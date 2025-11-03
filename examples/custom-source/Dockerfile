FROM pulumi/pulumi:3.205.0-nonroot@sha256:a1e2e39b014cd6807bae8e431b21146cabced856201fb5755aab4c0cb64f945c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py