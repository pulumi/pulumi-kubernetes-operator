FROM pulumi/pulumi:3.219.0-nonroot@sha256:4af249f2d807b202317f78b16d8fba3055ef4fbd8231ba212dfd6bc6fbca26d9

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py