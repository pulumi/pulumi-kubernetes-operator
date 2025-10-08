FROM pulumi/pulumi:3.201.0-nonroot@sha256:e1628031c873731da5be57417f033dbf27f0028e534d51125c2ed60116c23dd0

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py