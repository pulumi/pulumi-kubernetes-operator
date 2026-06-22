FROM pulumi/pulumi:3.247.0-nonroot@sha256:03442b4b4a52843ec1586872530a2f93532774b288ba9cb276023d9d6b11265d

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py