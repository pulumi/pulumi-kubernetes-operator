FROM pulumi/pulumi:3.191.0-nonroot@sha256:b6c9040709613bbfa8391ac90b697930a08671be5eef96f7be50e3bf10d27797

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py