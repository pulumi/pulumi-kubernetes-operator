FROM pulumi/pulumi:3.233.0-nonroot@sha256:bd9bf13d093c58dca07a3c271efa61dabf7d21959c871e6c1ea96aea680d03ae

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py