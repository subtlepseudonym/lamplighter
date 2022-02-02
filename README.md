# Lamplighter

This is a library, cron, and REST server for smoothly setting color and power states on LIFX bulbs. 

## Running the server

```bash
make docker
docker run \
	--name lamplighter \
	--publish 9000:9000 \
	--env "TZ=America/New_York" \
	lamplighter:$TAG
```
Note that `$TAG` should be replaced with the latest version tag. The docker make recipe tags images with the latest release specifically.

A different version tag can be specified like so:
`make docker BUILD=latest`

## Development

When running the recipes included in the makefile, you may want to have `vtag` in your PATH. The recipes will work without it, but the resulting binary will not include version information. You can get `vtag` by cloning [subtlepseudonym/utilities](https://github.com/subtlepseudonym/utilities), running `make build`, and copying `bin/vtag` into your PATH.
