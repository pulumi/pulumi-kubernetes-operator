FROM pulumi/pulumi:3.231.0-nonroot@sha256:b28c2c5a774eb9333fdfc17817b02d9d4acf66f24b9f2f2c8e5b6d4876833d9c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py