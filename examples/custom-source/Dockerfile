FROM pulumi/pulumi:3.259.0-nonroot@sha256:2853a2aef1875e9389b64bb47513eff589df8b962e8f295a015fd88411ce809d

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py