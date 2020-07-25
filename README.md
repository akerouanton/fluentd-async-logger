# fluentd-async-logger

This nonofficial Fluentd logging driver for Docker provides a way to reliably
use async mode on Docker v19.03 and v20.05. This is a workaround for
[#40063](https://github.com/moby/moby/issues/40063).

Most of the code here directly come from https://github.com/moby/moby/blob/f6a5ccf492e8eab969ffad8404117806b4a15a35/daemon/logger/fluentd/fluentd.go

## How to install?

You can install this plugin with:

```
$ docker plugin install --alias fluentd-async akerouanton/fluentd-async-logger:v0.2
```

Then you can optionally define it to be your default logging driver by updating
your `/etc/docker/daemon.json` (you need to restart your daemon after that):

```json
{
  "log-driver": "fluentd-async",
  "log-opts": {
      "fluentd-address": "tcp://1.2.3.4:24224/"
  }
}
```

**NOTE:** If you have running containers set to use the default `log-driver`
defined in your `daemon.json` and want to switch to `fluentd-async`, you need
to update your `daemon.json` file and then recreate those containers.

## How to use?

Once installed, you can either set it as your default logger as specified above
or you can define it on specific containers.

With Docker:

```bash
$ docker run \
    --log-driver fluentd-async \
    --log-opt fluentd-address=tcp://1.2.3.5:24224/ \
    ...
```

With docker-compose:

```yaml
services:
  nginx:
    logging:
      driver: fluentd-async
      options:
        fluentd-address: tcp://1.2.3.4:24224/
```

This logger exposes pretty much the same options as the official Fluentd log
driver, see [here](https://docs.docker.com/config/containers/logging/fluentd/).
However, it has two differences:

1. The `fluentd-async-connect` option is not available since this driver is only
fixing bugs related to the async mode. You should use the official fluentd log
driver if you need sync mode ;

2. The `fluent-force-stop-async-send` option has been added and defaults to
`true`. This new option tells the driver to discard pending logs to close as
soon as possible.

## How to debug?

To debug this plugin, you have to set `DEBUG` env var to `true` and `LOG_LEVEL`
to `debug`. Then you can curl its UNIX socket to get traces of its goroutines
and you can see its logs on dockerd output (generally via `journalctl`):

```bash
$ docker plugin set fluentd-async \
    DEBUG=true \
    LOG_LEVEL=DEBUG
$ export PLUGIN_ID=$(docker plugin ls --no-trunc | awk '/akerouanton\/fluentd-async-logger/ {print $1}')
$ sudo curl -H "Content-Type: application/json" \
    -XPOST -d '{}' \
    --unix-socket /var/run/docker/plugins/${PLUGIN_ID}/fluentd-async.sock \
    http://foobar/pprof/trace # The dummy foobar hostname is used here because
                              # curl requires a hostname even when using unix sockets.
$ sudo journalctl -xfu docker | grep ${PLUGIN_ID}
```

## How to work on this?

You can create a new `devel` plugin release with `make plugin` and you can
install it with `make install`.

To release a new version you have to use: `PLUGIN_VERSION=vX.Y make release`.

To test the plugin and confirm it doesn't suffer from the issue describe in #40063:

```bash
$ docker run --name repro -d -p 80:80 --log-driver akerouanton/fluentd-async-logger:devel nginx
$ curl http://localhost
$ docker stop repro # This should be done in a couple of seconds
```
