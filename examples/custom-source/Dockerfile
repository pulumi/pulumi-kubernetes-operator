FROM pulumi/pulumi:3.229.0-nonroot@sha256:6407eae762f32b8bb074d5b328e582438a833592f745f2c593a00b4de6a74b43

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py