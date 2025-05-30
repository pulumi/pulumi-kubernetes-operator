FROM pulumi/pulumi:3.173.0-nonroot@sha256:6b926d6bda48a10b78ff0324365bb387e850b06eaecbd820ac72bbb2daff9f66

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py