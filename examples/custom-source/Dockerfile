FROM pulumi/pulumi:3.214.1-nonroot@sha256:df76766feac68ff1782084893b768e0201fb69668bb759aad2335c68da89671f

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py