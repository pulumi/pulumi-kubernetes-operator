FROM pulumi/pulumi:3.194.0-nonroot@sha256:b41ca28d78a45c6f786193da8283d138ff964f341f564fa6b5485cb20740382a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py