FROM pulumi/pulumi:3.244.0-nonroot@sha256:a08f88a546f31820e8eb381544773f27e8964a15c2777a419954ff2af46dd09c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py