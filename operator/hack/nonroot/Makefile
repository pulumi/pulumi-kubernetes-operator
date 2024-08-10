
.PHONY=image

IMG ?= pulumi/pulumi:3.128.0

image:
	docker build -t $(IMG)-nonroot --build-arg "BASE_IMAGE=$(IMG)" .