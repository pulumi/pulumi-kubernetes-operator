FROM pulumi/pulumi:3.144.1-nonroot@sha256:8405e112404d1900afae5420074e5955a96f4a7069f5c08099a8e8f81677b4c4

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py