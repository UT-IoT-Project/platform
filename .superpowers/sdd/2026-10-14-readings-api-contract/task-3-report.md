# Task 3 report: Preserve PIR IDs in aggregate readings

## Status

Completed the OpenAPI contract update in `docs/api/openapi.yaml` only. No runtime code was changed, and no Go tests were run.

## Changes

- Added the closed `PIRValue` object schema with required integer `ID` and numeric `value` properties.
- Changed `PIRReading.values` to reference `PIRValue`; `PIRReading` remains closed to additional properties.
- Updated the populated PIR response example to show three identified readings at the same timestamp: IDs 1, 2, and 3 with values 1, 0, and 1.
- Reworded the PIRReading description to state that each sensor ID is preserved with its value.
- Preserved the existing request sensor list, other sensor response schemas, and response status behavior.

## Validation

- Ruby's standard YAML parser successfully parsed `docs/api/openapi.yaml`.
- A structural check confirmed `PIRValue` requirements, property types, and closed-object behavior; the `PIRReading.values` reference; the closed `PIRReading` schema; and the three-item populated example.
- `git diff --check` passed.
- PyYAML was unavailable in the environment; no package was installed because the Ruby YAML parser provided the required validation.

## Concerns

No known contract concerns. This changes documentation only; endpoint runtime behavior remains outside this task.

## Commit

Pending at report creation; recorded in the task completion message after commit.
