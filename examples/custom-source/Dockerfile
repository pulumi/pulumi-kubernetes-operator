FROM pulumi/pulumi:3.203.0-nonroot@sha256:48ae8b528c5a05239374f59e099357eb288b3d5b821b2ae505d56d4efb8c5017

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py