FROM pulumi/pulumi:3.159.0-nonroot@sha256:acd49ad419bde0f5be16c9fb72301217c44926d9954a603da53f902ce25cc2fd

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py