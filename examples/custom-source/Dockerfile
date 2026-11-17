FROM pulumi/pulumi:3.276.0-nonroot@sha256:a57f8754c304fd5f8dea12d87683f72399577378b6e0028c4480517d16cb9108

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py