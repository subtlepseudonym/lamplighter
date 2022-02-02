# Lamplighter

This is a library, cron, and REST server for smoothly setting color and power states on LIFX bulbs. 


## Running the server

We can create the docker image and container, but we'll need to define a config file before we run anything.
```bash
make docker
docker create \
	--name lamplighter \
	--publish 9000:9000 \
	--env "TZ=America/New_York" \
	--volume "/path/to/local/config:/config" \
	lamplighter:$TAG
```
Note that `$TAG` should be replaced with the latest version tag. The docker make recipe tags images with the latest release specifically.

A different version tag can be specified like so:
`make docker BUILD=latest`

This project can also be compiled and run natively by calling `make build` and running the resulting binary, but these instructions will only cover running via docker.

### Configuration

A config file should be included at `config/lamp.cfg`. The JSON structure of the config is defined by `cmd/lamplighter/config/config.go`. This contains the location, names, and network information of your bulbs as well as their schedules for changing states. Lamplighter uses a (very lightly) modified cron parser for scheduling state transitions. In addition to the cron formats [supported by robfig/cron](https://github.com/robfig/cron/#background---cron-spec-format), lamplighter can parse the format `@sunset $OFFSET` where `$OFFSET` is a duration string parseable by go's `time.ParseDuration` function.

For example:
```json
{
	"location": {
		"latitude": 0.0,
		"longitude": 0.0
	},
	"devices": {
		"lamp": {
			"ip": "1.1.1.1",
			"mac": "00:00:00:FF:FF:FF"
		}
	},
	"jobs": {
		{
			"schedule": "@sunset -1h",
			"device": "lamp",
			"brightness": 100,
			"kelvin": 3000,
			"transition": "1m"
		},
		{
			"schedule": "@sunset +2h",
			"device": "lamp",
			"brightness": 100,
			"hue": 180,
			"saturation": 50,
			"transition": "500ms"
		},
		{
			"schedule": "0 2 * * *",
			"device": "lamp",
			"brightness": 0,
			"transition": "15s"
		}
	}
}
```

### Making HTTP requests

Once the config file is defined, start the container. You should see some helpful log messages to indicate that the defined bulbs have been detected and are communicating with the server.
```bash
docker start lamplighter
docker logs --follow lamplighter
```

By default, lamplighter listens for requests on port 9000.

Endpoints are generated for every bulb in your config file. When a request is received, the specified bulb is transitioned to the desired state immediately (as opposed to the scheduled approach using cron).

Using the example config above, the following requests are equivalent to the scheduled jobs:
```bash
curl "http://localhost:9000/lamp?brightness=100&kelvin=3000&transition=1m"
curl "http://localhost:9000/lamp?brightness=100&hue=180&saturation=50&transition=500ms"
curl "http://localhost:9000/lamp?brightness=0&transition=15s"
```

A status endpoint for each bulb can be used to get the device's current state:
```bash
curl "http://localhost:9000/lamp/status"
```

The entries endpoint lists cron entries for upcoming jobs:
```bash
curl "http://localhost:9000/entries"
```


## Development

When running the recipes included in the makefile, you may want to have `vtag` in your PATH. The recipes will work without it, but the resulting binary will not include version information. You can get `vtag` by cloning [subtlepseudonym/utilities](https://github.com/subtlepseudonym/utilities), running `make build`, and copying `bin/vtag` into your PATH.
