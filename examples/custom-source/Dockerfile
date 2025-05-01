FROM pulumi/pulumi:3.167.0-nonroot@sha256:0cad6bcc663974c9ee047f4d460cb29746857f1f2d95a32271d4e4d0a59cc165

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py