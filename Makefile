CMDS=satellite-object-storage-plugin

all: reltools build

include release-tools/build.make

REGISTRY_NAME=quay.io/satellite-object-storage-plugin
IMAGE_TAGS=canary