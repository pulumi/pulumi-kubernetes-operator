FROM pulumi/pulumi:3.147.0-nonroot@sha256:38f4f9122e7418e38896dc6c51aacbcb84896e4251c2bc3dbcf393d27668290a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py