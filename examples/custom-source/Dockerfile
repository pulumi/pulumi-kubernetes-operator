FROM pulumi/pulumi:3.266.0-nonroot@sha256:2f80379f3290d046f0f7c0639f135f678d8cb945a32841c912d8e934fa073b1c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py