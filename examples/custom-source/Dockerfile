FROM pulumi/pulumi:3.186.0-nonroot@sha256:67dff4aedc8a3702bf7a2d0010d6c17e4068a7eb4126b2c24b934f7b87d4a259

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py