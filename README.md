# contracts

What every app of the org answers, written down once. Each answer has a schema and a sample. An app and its readers test against the same files, so a change that breaks a reader is caught early.

The rules and the layout of the folders are in `constitution/tech-stack.md`; the purpose and the words are in `constitution/mission.md`.

## What is here

- `envelope/v1/`, `error/v1/`, `event/v1/`: the shapes every answer, error and event shares.
- `mankit/v1/`: what mankit serve answers for every app: the record a detached `serve` writes, the `servers` answers, the `<product>.stream.gapped` event a client of `GET /api/events` gets when it missed events, and the change queue's ticket and the `<app> queue` answer. Each app lists that event in its own `events.asyncapi.yaml`.
- `<app>/<version>/`: one app's contract, named by its product ID. `designer/v1/` is the first.
- `apiversion/v1/forms.yaml`: the one table of every apiVersion form, old and new. A reader that meets an old form can say it is old, and the row names the upgrade command (`hubman migrate <app> <folder>`) and the current form.

An answer has five fields: `apiVersion`, `kind`, `ran`, `data` and `errors`. `ran` says where it came from: the name, the app, its version and the contract version. A reader asks for a version with `--contract v1` or the `Accept-Contract: v1` header. An app that cannot answer it gives the error `unsupported_contract` and lists the versions it supports.

An event is a CloudEvents 1.0 event. An app lists its events in `events.asyncapi.yaml` (AsyncAPI 3.0), and each event has a schema and a sample.

## The change queue on disk

Every app that changes a served thing shares one queue in `<served folder>/<app folder>/.queue/` (ADR-S14), and two app versions may share it at once. So within `mankit/v1` these rules never change; a change to them is a new version.

- The folder and its `bad/` are mode 0700. It holds `lock`, `turn`, `bad/`, `.gitignore` (holding `*`) and the tickets, `<id>.json` (`ticket.schema.json`). Names starting with a dot are not tickets and are left alone; anything else that is not a readable ticket is moved to `bad/`, never over a file already there, and said.
- A lock is exclusive and never waited on with the system's blocking call: `flock(LOCK_EX|LOCK_NB)` on the whole file on Linux and macOS, `LockFileEx` with `LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY` on one byte at offset 2^62 on Windows, so the file's content stays readable.
- `lock` is held while a ticket is written, the folder is read, or a ticket is removed. A ticket is written whole (through a temporary dot-file renamed into place) and locked before `lock` is given up.
- A ticket's file is locked by its process from writing to removing. A reader that can take that lock knows the process has ended and removes the ticket.
- A process takes `turn` only while it holds `lock` and its ticket is first, holds it until its change is written and read back, and writes its ticket's id in `turn` while it holds it.
- A ticket is touched every 5 seconds while it waits and runs. A ticket whose lock is held but which was not touched for 30 seconds is stale and moved to `bad/`.
- A ticket's id is 20 digits, after every id in the line; the lowest goes first.

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
