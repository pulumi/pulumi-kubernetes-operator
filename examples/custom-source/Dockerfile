FROM pulumi/pulumi:3.262.0-nonroot@sha256:a8accb6429f15ac4f0cbc5d055bb50eec158100f62a3baa38ab49615b96388ee

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py