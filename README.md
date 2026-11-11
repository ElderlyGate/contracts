# contracts

What every app of the org answers, written down once. Each answer has a schema and a sample. An app and its readers test against the same files, so a change that breaks a reader is caught early.

The rules and the layout of the folders are in `constitution/tech-stack.md`; the purpose and the words are in `constitution/mission.md`.

## What is here

- `envelope/v1/`, `error/v1/`, `event/v1/`: the shapes every answer, error and event shares.
- `mankit/v1/`: what mankit serve answers for every app: the record a detached `serve` writes, the `servers` answers, and the `<product>.stream.gapped` event a client of `GET /api/events` gets when it missed events. Each app lists that event in its own `events.asyncapi.yaml`.
- `<app>/<version>/`: one app's contract, named by its product ID. `designer/v1/` is the first.
- `apiversion/v1/forms.yaml`: the one table of every apiVersion form, old and new. A reader that meets an old form can say it is old, and the row names the upgrade command (`hubman migrate <app> <folder>`) and the current form.

An answer has five fields: `apiVersion`, `kind`, `ran`, `data` and `errors`. `ran` says where it came from: the name, the app, its version and the contract version. A reader asks for a version with `--contract v1` or the `Accept-Contract: v1` header. An app that cannot answer it gives the error `unsupported_contract` and lists the versions it supports.

An event is a CloudEvents 1.0 event. An app lists its events in `events.asyncapi.yaml` (AsyncAPI 3.0), and each event has a schema and a sample.

## Use from Go

The package `github.com/ElderlyGate/contracts` embeds every schema, sample and table, so an app's tests check its real answers here, without a copy:

```go
err := contracts.Validate("envelope/v1/envelope.schema.json", body)
```

`contracts.FS` holds the files themselves.

## Test

```
go test ./...
```

The tests check that:

- every schema is valid against draft 2020-12;
- every sample meets its schema, and every schema has a sample;
- every event in an `events.asyncapi.yaml` is a valid event, with its schema and sample;
- every file in `testdata/bad/` is refused by its schema;
- the apiVersion table lists each form once; an old row has its upgrade command and points to a current form.
