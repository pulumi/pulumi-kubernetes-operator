FROM pulumi/pulumi:3.192.0-nonroot@sha256:e45620e63c9dc857a1265a90eed460aeb9c6600bc1a9fafa8bc4df60e348b780

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py