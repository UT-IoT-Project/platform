# Readings HTTP Boundary Design

## Intent and scope

Register the one GET readings route declared in `docs/api/openapi.yaml` so clients can send the documented path and query parameters. This change covers the HTTP adapter only. A core readings service is not yet implemented or wired, so valid requests cannot return readings in this change.

## Approach

The chosen approach is an HTTP boundary that binds and validates the request, then returns the documented generic 500 response until a service is available. A full endpoint wired to InfluxDB would also need a core use case, dependency wiring, and a read result mapper; that is outside this change. A route-only stub would leave the request contract untested and was not chosen.

## HTTP behavior

- `NewServer` calls `registerRoutes` inline on the new `Server` value. `registerRoutes` installs `GET /api/v1/readings/:building/:roomNumber` and returns the server.
- The handler lives in its own file under `internal/driver/http/`. Its request DTO lives under `internal/driver/http/request/` (the repository's documented spelling for the requested folder).
- Echo binds `building` and `roomNumber` from path parameters, and `start_time`, `end_time`, and repeated `sensors` query parameters to the DTO.
- Require both timestamps in `YYYY-MM-DD HH:mm:ss.SSS` form in `Europe/Amsterdam` local time and reject impossible calendar values. Convert valid times to UTC `time.Time` values at this boundary for a future core query. Require at least one sensor. Accept only the exact case-sensitive names in the current OpenAPI file.
- Validation failures return status 400 with the exact documented message and empty `data` object. Validation order is timestamp, empty sensor list, then unknown sensor.
- A valid request returns status 500 with `Internal server error.` and an empty `data` object until the core service is connected. The handler must not claim a successful read or invent data.

## Constraints and follow-up

Keep Echo types out of the core and do not add database access to the handler. The OpenAPI 200 and 404 responses remain future behavior. The request timezone is `Europe/Amsterdam`; daylight saving time offsets are handled by Go's timezone database. Since an input contains no UTC offset, an ambiguous hour during the autumn clock change cannot identify which occurrence the caller intended. Reject ambiguous and nonexistent local times as invalid timestamps rather than guessing or silently shifting them. Update the OpenAPI wording to name the timezone.
