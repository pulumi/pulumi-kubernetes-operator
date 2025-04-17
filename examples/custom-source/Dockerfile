FROM pulumi/pulumi:3.163.0-nonroot@sha256:e2a4e1ab0410074f13a60727c3067eb7b9cbe7a0cb945c2e09ea497aea628a4a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py