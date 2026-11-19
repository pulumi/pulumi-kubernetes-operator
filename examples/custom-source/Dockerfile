FROM pulumi/pulumi:3.277.0-nonroot@sha256:02451ea9879d871a999cd71c92c52f848355d6e94233b836491ae794547ff625

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py