FROM pulumi/pulumi:3.148.0-nonroot@sha256:f6cd13cb7d1bfa7cdafb6f89bab28c4ef7e8e6c15fb80965f0e510a2605621cd

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py