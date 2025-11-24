FROM pulumi/pulumi:3.208.0-nonroot@sha256:750c333541905ef5bea484590b75191dd7479268661bff25c75c0ecd77fe861a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py