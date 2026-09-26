FROM pulumi/pulumi:3.265.0-nonroot@sha256:de37420273b99294afdd15a6841b1885b25be551e2b537e986229177e9f8449f

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py