FROM pulumi/pulumi:3.241.0-nonroot@sha256:9849179d3e854ef896a7011a71c654c353b2a668c3ed36bbea90f218a5045e8d

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py