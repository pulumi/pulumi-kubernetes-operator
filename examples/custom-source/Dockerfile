FROM pulumi/pulumi:3.197.0-nonroot@sha256:f9de46431ab60872c874f2d1ca7155327a35876ff94d987cde357e38229ad135

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py