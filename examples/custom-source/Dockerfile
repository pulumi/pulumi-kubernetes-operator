FROM pulumi/pulumi:3.239.0-nonroot@sha256:4e0e64414bdfb9e6ad0d946bf77aeced8cfc3e4f1603b2034518639c3e2ee199

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py