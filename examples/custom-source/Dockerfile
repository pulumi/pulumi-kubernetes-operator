FROM pulumi/pulumi:3.232.0-nonroot@sha256:f224f2425ca1717b20bdd987d041e839eb8d2ffc2de2c55a0fd8b023884f4e4c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py