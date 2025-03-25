FROM pulumi/pulumi:3.158.0-nonroot@sha256:8198238a44098b4bfafc347fe99c193303b5169cdd25705b166080ef9fe8c148

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py