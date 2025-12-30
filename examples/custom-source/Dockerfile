FROM pulumi/pulumi:3.214.0-nonroot@sha256:0f2a60026af656580859dab8f3f79f9d8c273c0586d8278f4c7871446f26e420

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py