# gotohp worker protocol

`gotohp-worker` exposes the Go core to external clients over standard input and output.

The worker reads one JSON-RPC request per line from stdin and writes one response or notification per line to stdout. Logs must go to stderr. The process accepts successive requests and exits when stdin reaches EOF.

## Requests

Each request uses JSON-RPC 2.0 fields:

```json
{"jsonrpc":"2.0","id":"1","method":"upload","params":{"paths":["/photos/a.jpg"],"options":{"Threads":2}}}
```

The first supported method is `upload`.

## Notifications

Upload progress is sent as JSON-RPC notifications associated with the request ID:

- `uploadStart`
- `totalBytes`
- `totalBytesDelta`
- `warning`
- `threadStatus`
- `fileResult`
- `albumProgress`
- `albumComplete`
- `albumError`
- `uploadStop`

The final response has the request ID and an `ok` result.

## Errors

Errors use the JSON-RPC error shape with stable codes such as `invalid_request` and `method_not_found`.

The protocol is transport independent. A future socket or named-pipe transport should preserve these messages and methods.
