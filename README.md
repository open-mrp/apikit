# apikit

The HTTP API framework behind [OpenMRP](https://github.com/open-mrp/api), extracted so other APIs can share it.

apikit is built to the forge.1 public API contract: one error object, keyset pagination, idempotency keys, version headers, expandable includes, and an OpenAPI spec generated from the endpoint definitions themselves.

> **Status:** early. Packages are being moved in from `open-mrp/api` and their APIs may change before v1.

## Install

```sh
go get github.com/open-mrp/apikit
```

## License

Apache 2.0. See [LICENSE](LICENSE).
