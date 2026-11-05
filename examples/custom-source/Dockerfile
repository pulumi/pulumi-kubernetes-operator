FROM pulumi/pulumi:3.274.1-nonroot@sha256:f49ca66d0583e9d479f16223c9524e16468ce3b5f2892c60c1ce12aec92d2fd5

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py