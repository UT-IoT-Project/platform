# Readings API Contract Design

## Goal

Define a Swagger/OpenAPI contract for clients to request selected sensor readings over a time range. This documents the API interface only; it does not implement an HTTP route, query service, or database adapter.

## Endpoint and request

- Method: `GET`
- Path: `/api/v1/readings/{start_timestamp}/{end_timestamp}`
- `start_timestamp` and `end_timestamp` are required path parameters formatted as `YYYY-MM-DD HH:mm:ss.SSS` in the server's configured local timezone. Spaces in URL path segments must be percent-encoded as `%20`.
- `sensors` is a required, nonempty query array represented with repeated keys. For example: `?sensors=PIR&sensors=CO2`.
- Supported sensor names are case-sensitive: `PIR`, `CO2`, `humidity`, and `temprature`. `PIR` requests all PIR readings. Any other sensor name returns HTTP 400.

A GET request body is not used. The request values are carried in the path and query string.

## Response

Every response contains:

- `status`: integer matching the HTTP status code.
- `message`: a string describing the result.
- `data`: object with one property per requested sensor.

For `CO2`, `humidity`, and `temprature`, each data property is an array of readings with `timestamp` (same format and timezone convention as the request) and numeric `value`.

For `PIR`, `data.PIR` is an array of timestamped objects. Each object has `timestamp` and `values`, an array of numeric readings for all PIR sensors at that timestamp. PIR identifiers are omitted from this response array.

For HTTP 200, `message` is `data retrieved successfully`. If a requested sensor has no readings in the range, its key remains in `data` with an empty array. An empty result still returns HTTP 200.

For HTTP 400 validation errors, `data` is an empty object and `message` is one of:

- `Invalid timestamp format.` when either timestamp does not match the required format.
- `Empty list of sensors are not allowed.` when `sensors` is missing or empty.
- `Unknown sensor name.` when a sensor name is not in the supported list.

Unexpected server errors use HTTP 500, the same response envelope, and an empty `data` object. The message is `Internal server error.`

## OpenAPI artifact

`docs/api/openapi.yaml` uses OpenAPI 3.1.0. It defines the endpoint parameters, response schemas, and examples for populated and empty results, PIR aggregation, each 400 validation error, and 500. It documents the supported sensor names and that URL path spaces are encoded as `%20`. The HTTP route remains unimplemented.
