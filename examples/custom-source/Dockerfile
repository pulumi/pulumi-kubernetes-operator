FROM pulumi/pulumi:3.188.0-nonroot@sha256:d639f2e258c43e40acf6233046b78bf80b84f0b566e8eec50e896349325496d9

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py