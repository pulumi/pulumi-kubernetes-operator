FROM pulumi/pulumi:3.161.0-nonroot@sha256:2c452940eb5fb8775f0949d561ed7dc3e08ebe587fb4647a8f5f93c2805e0479

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py