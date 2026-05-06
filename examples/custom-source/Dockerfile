FROM pulumi/pulumi:3.235.0-nonroot@sha256:86a16d59c65cb38e3d08fa46d587e4a8a6ce0c420404b82413e34f7b7cd91760

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py