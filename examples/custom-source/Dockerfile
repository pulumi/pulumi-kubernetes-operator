FROM pulumi/pulumi:3.171.0-nonroot@sha256:17f98cbc24982c710f72dceaf9a4b2883fc563345c1854b5f1feaa83dc77f08d

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py