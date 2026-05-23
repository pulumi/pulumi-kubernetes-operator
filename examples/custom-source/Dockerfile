FROM pulumi/pulumi:3.243.0-nonroot@sha256:87691c0d41d2a09430daf58113999e19168c57a92b8291c1be42c71768583cd8

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py