FROM pulumi/pulumi:3.181.0-nonroot@sha256:0a54efa5b5690e7aff973a28eed8df0a28db43a9f4a65e8ff9ab6f3dd65fc6dc

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py