FROM pulumi/pulumi:3.217.0-nonroot@sha256:574c7dd52ebaff3a46349c72085dddaa6c766148953515df44f9741b9338d17d

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py