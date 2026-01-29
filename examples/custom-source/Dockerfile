FROM pulumi/pulumi:3.218.0-nonroot@sha256:f4bd58b8c894543d282ab9196d505fd6e99f7af8a31649532720cfebf23ea24b

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py