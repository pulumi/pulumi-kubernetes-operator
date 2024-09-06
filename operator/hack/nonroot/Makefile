
.PHONY=image

IMG ?= pulumi/pulumi:3.130.0

image:
	docker build -t $(IMG)-nonroot --build-arg "BASE_IMAGE=$(IMG)" .