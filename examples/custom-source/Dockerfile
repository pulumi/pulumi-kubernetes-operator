FROM pulumi/pulumi:3.211.0-nonroot@sha256:19d1b26faf23dccb08b6701a6c8add2f4110aca24b7e166bf3ca86590eb3157b

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py