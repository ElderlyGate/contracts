# Tech Stack

## Stack

- **CT-STACK-1** 2026-11-11 · Schemas are JSON Schema, draft 2020-12.
- **CT-STACK-2** 2026-11-11 · The tests are Go. They check every sample against its schema, and every schema against the draft.

## Layout

```
envelope/v1/        the envelope every answer shares
error/v1/           the one error format
event/v1/           the event envelope every event shares (CloudEvents)
apiversion/v1/      every apiVersion form, the old forms it replaces and its kinds (one table for every app)
<app>/<version>/    one app's contract: <kind>.schema.json, samples/<kind>.json and events.asyncapi.yaml
```

## Rules

- **CT-RULE-1** 2026-11-11 · A version's schemas do not change once an app has released it. A change that a reader would notice is a new version.
- **CT-RULE-2** 2026-11-11 · Every schema has at least one sample. The app's own tests check that its real answer matches each sample, so a sample can be written before the app gives that answer.
- **CT-RULE-3** 2026-11-11 · A new version comes with its upgrade step from the one before, tested with samples of both versions.
- **CT-RULE-4** 2026-11-11 · An app's own shapes allow no fields beyond those listed. Two shapes stay open: a design's `spec`, written by its user, and the `spec` of a record that two app versions share at once (ADR-S15), so an older reader accepts fields a newer app adds.
