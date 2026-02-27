FROM pulumi/pulumi:3.224.0-nonroot@sha256:f92e01e3210717ec22061146d1da7a7e7a0ec341456797e8277728124e654b58

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py