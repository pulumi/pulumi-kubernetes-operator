FROM pulumi/pulumi:3.223.0-nonroot@sha256:6384fc8f4e924ad1f64be1a51c032cdfdb36faf0e4b44154e7279d5fe1c2ed90

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py