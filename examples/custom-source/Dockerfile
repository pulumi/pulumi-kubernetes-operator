FROM pulumi/pulumi:3.237.0-nonroot@sha256:a071f124df34cd4d8d4bedaee1e772c3382701e5176f6879bb1cc7bfe508dabc

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py