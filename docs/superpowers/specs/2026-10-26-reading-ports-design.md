# Reading Ports Design

## Intent

Define core reading models and ports for one request that selects multiple sensors in one room and time range. The repository adapter will later execute one database query for the batch, including PIR readings when requested.

## Design

- Replace the generic single-value and multiple-value models with `ScalarReading`, `PIRValue`, and `PIRReading`. `PIRValue` keeps its ID attached to its value; scalar readings have no PIR-only field.
- `ReadingsResult` holds scalar series in `map[domain.SensorType][]domain.ScalarReading` and PIR readings in `[]domain.PIRReading`. Requested sensors with no data can be represented by empty slices.
- `ReadingsQuery` contains building, room, start, end, and selected sensor types.
- `ReadingsService` and `ReadingsRepository` each expose one `GetReadings(context.Context, ReadingsQuery) (ReadingsResult, error)` method. The service will own application rules; the repository will own the future query and result mapping.
- Keep the result free of a `LocationExists` flag. There is no authoritative room registry today, so an empty range cannot establish whether the location exists.

## Constraints

- Keep core packages independent of HTTP, InfluxDB, and MQTT libraries.
- Preserve the user's uncommitted sensor additions. Correct the three misspelled enum identifiers before using them in new interfaces.
- Do not add an HTTP route, service implementation, repository adapter, database query, or speculative tests for declarations.
- The OpenAPI 404 contract remains unresolved until location existence has a trustworthy source.

## Approaches

- **Chosen:** separate scalar and PIR models, with one typed batch result and one method at each port. This keeps PIR identity explicit and avoids a separate call per sensor.
- One struct field per sensor would be type-safe but repeats the same scalar shape eight times.
- One flat reading type with an optional PIR ID would be smaller but permits PIR-only fields on other sensors.
