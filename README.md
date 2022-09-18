# transcode is markup language conversion library

[![Go Reference](https://pkg.go.dev/badge/github.com/chanced/transcode.svg)](https://pkg.go.dev/github.com/chanced/transcode)

transcode is a go library that will convert markup languages (currently JSON and YAML) from one form to another.

## Motivation

While it is possible to decode JSON as YAML, doing so comes with caveats. YAML
is a superset of JSON and so unmarshaling a YAML object into `interface{}` will
result in `map[interface{}]interface{}`. When you need to work with the raw data
or would like to work with JSON Schema, this can be problematic.

Encoding JSON as YAML is easily accomplishable with standard tooling. However,
this library will be faster than unmarshaling JSON as `interface{}`, and then
remarshaling as YAML. It also provides some very minimal formatting.

There are some drawbacks to using transcode though:

-   YAML aliases are not supported
-   All keys remain strings when transcoding into YAML
-   YAML comments are obviously lost

## License

MIT
