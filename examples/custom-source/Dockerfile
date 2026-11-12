FROM pulumi/pulumi:3.275.0-nonroot@sha256:63d375a4bab08115a2aff4defe44140b03ef39e3747dd6c775314311620937d7

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py