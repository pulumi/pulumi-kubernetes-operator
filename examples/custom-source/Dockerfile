FROM pulumi/pulumi:3.270.0-nonroot@sha256:14c1fd4dfddd112a11dd601c86fa59512e6313eea4a0d292b25c7bc11c9c1a7d

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py