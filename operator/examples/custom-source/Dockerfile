FROM pulumi/pulumi:3.145.0-nonroot@sha256:9b8c142eabcb59a8ecac5e06204bb0b632573ef53b74b88adc7768c275b4a46f

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py