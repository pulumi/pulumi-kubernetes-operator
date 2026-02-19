FROM pulumi/pulumi:3.222.0-nonroot@sha256:72022a3f055fd909f6d94c93f11fa2a06968152f1601861cfc3f2162d193865c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py