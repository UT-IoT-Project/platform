# Task 1 Report: Publish the OpenAPI Contract

## Status

Complete. Added the OpenAPI 3.1.0 contract and updated the README to make clear that the contract exists while the HTTP route is still unimplemented. No runtime code was changed.

## Files changed

- `docs/api/openapi.yaml`
  - Defines `GET /api/v1/readings/{start_timestamp}/{end_timestamp}`.
  - Declares required timestamp path parameters using `YYYY-MM-DD HH:mm:ss.SSS` and the server's configured local timezone.
  - Declares required repeated-key `sensors` array parameters using `style: form` and `explode: true`, with open string sensor names.
  - Documents `%20` encoding for spaces in path timestamps and gives a concrete URL example with repeated `sensors` keys.
  - Defines dynamic sensor data, reading objects, success and error envelopes, and populated, empty, 400 validation, and 500 examples.
  - The 400 examples use the exact approved messages; examples use a `status` equal to their HTTP response code.
- `README.md`
  - Points to `docs/api/openapi.yaml` and states that the documented readings route remains unimplemented.

## Validation

- Ruby YAML parser successfully loaded `docs/api/openapi.yaml`.
- A structural check confirmed OpenAPI version `3.1.0`, the route and required parameters, repeated-key array serialization, `%20` documentation, both exact 400 messages, and matching `status` values across all five response examples.
- `git diff --check` completed without errors.
- Go tests were not run, as requested.

## Concerns

No known contract concerns. Validation used YAML parsing plus targeted structural assertions; no dedicated OpenAPI linter was run.
