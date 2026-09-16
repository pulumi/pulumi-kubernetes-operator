FROM pulumi/pulumi:3.263.0-nonroot@sha256:470b9e17d7544af3fd643fb5675849ff912639d0e7caa1a92852d7649ca0dad0

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py