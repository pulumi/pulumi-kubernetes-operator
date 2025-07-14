FROM pulumi/pulumi:3.182.0-nonroot@sha256:0605775110b20123aeb3f20ec579b474060e225355c17763d2e29c4e2e139c24

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py