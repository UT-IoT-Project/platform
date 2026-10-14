# Readings API Contract Design

## Goal

Define a Swagger/OpenAPI contract for clients to request selected sensor readings over a time range. This documents the API interface only; it does not implement an HTTP route, query service, or database adapter.

## Endpoint and request

- Method: `GET`
- Path: `/api/v1/readings/{start_timestamp}/{end_timestamp}`
- `start_timestamp` and `end_timestamp` are required path parameters formatted as `YYYY-MM-DD HH:mm:ss.SSS` in the server's configured local timezone. Spaces in URL path segments must be percent-encoded as `%20`.
- `sensors` is a required query array represented with repeated keys. For example: `?sensors=PIR1&sensors=CO2`.
- Sensor names are open strings to support dynamic PIR identifiers and named measurements such as `CO2`, `humidity`, and the currently configured `temprature` field.

A GET request body is not used. The request values are carried in the path and query string.

## Response

A successful response contains:

- `status`: integer matching the HTTP status code.
- `data`: object with one property per requested sensor. Each property is an array of reading objects with `timestamp` (same format and timezone convention as the request) and numeric `value`.

For errors, the response uses the same envelope and an empty `data` object. The contract documents request validation errors as HTTP 400 and unexpected server errors as HTTP 500.

## OpenAPI artifact

Create `docs/api/openapi.yaml` using OpenAPI 3.1.0. Include the endpoint parameters, success and error schemas, response examples, and a note that URL path spaces are encoded as `%20`. Keep the current lack of an implemented route accurate in the project status documentation.
