FROM pulumi/pulumi:3.236.0-nonroot@sha256:b8abf6cb9097a9bf36b45afb69483075a930f922874d049a94e814ea9ad6f33a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py