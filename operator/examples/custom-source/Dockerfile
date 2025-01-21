FROM pulumi/pulumi:3.146.0-nonroot@sha256:d5cd372b71b81323cf5ca7fab01a86d3d6748ad3436778fdd020e766e98a9e81

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py