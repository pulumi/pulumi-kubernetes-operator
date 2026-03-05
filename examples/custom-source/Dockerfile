FROM pulumi/pulumi:3.225.1-nonroot@sha256:bd5632affd99d009c26a35605d13d411345599b34eeeb8b2c459ccfc034e7913

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py