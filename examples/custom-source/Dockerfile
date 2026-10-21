FROM pulumi/pulumi:3.271.0-nonroot@sha256:491f0ec0c0f7933fa2fb793dadb7cbbf35adfbcb6cc58bbf48a5f9b1119ca7f0

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py