FROM pulumi/pulumi:3.162.0-nonroot@sha256:bd07fd36bb57cd482930a3cc7b1e1652a4df8bc8eef348ac25279c06085ede62

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py