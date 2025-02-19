FROM pulumi/pulumi:3.150.0-nonroot@sha256:84e9b5255d3a4339849f4c95d27a68df891209285faf5e901f8026f85d158549

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py