FROM pulumi/pulumi:3.183.0-nonroot@sha256:b015ec67b225c9b5a228d647aeb5056d23801a3e3c9df39a73125845e4c5fa18

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py