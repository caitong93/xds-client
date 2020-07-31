## xds-client

Client for [xDS protocol](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol#xds-protocol), for debugging

### Get Started

```
make compile
./bin/xds-client  -pilot-address ${PILOT_POD_IP}:15010 -clients 1
```

### Use as Library

```
import (
    driver "github.com/caitong93/xds-client/driver"
)
```

See `cmd/xds-client/main.go` for details