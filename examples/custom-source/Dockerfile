FROM pulumi/pulumi:3.273.0-nonroot@sha256:527c62bcfdd1a0e2d23d54b1d3f2428c66f6a2759a753dd2363302da186d5cdc

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py