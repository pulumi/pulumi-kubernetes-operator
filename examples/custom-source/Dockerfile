FROM pulumi/pulumi:3.226.0-nonroot@sha256:eaea084c5627933247f7bc0cba1d0a9230d7d8b6ad494909520bfe26d9dfd3b9

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py