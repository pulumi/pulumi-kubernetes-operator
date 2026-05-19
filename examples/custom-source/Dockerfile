FROM pulumi/pulumi:3.242.0-nonroot@sha256:d7d380ec884bd02e44c36e50b19eaaa95daaa2c5e60357f86261c5bdf42d637a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py