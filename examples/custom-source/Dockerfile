<<<<<<< HEAD
FROM pulumi/pulumi:3.202.0-nonroot@sha256:87b000ac99932e5a75bd35192a2ac6bcf79b2952b6041c93c01b1bf03bb86937
=======
FROM pulumi/pulumi:3.202.0-nonroot@sha256:4e580aefa911a7ef187cb68cae5c7d5e74765929239d6fba018288c13515b971
>>>>>>> b520d1b (Upgrade to Pulumi SDK v3.202.0 and enable JSON configuration support)

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py