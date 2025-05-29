FROM pulumi/pulumi:3.172.0-nonroot@sha256:a1985bff504d712756d28315abd6de8e7cba41f7cd9360ec8823a5a17c32ca98

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py