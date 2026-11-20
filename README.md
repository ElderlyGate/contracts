# contracts

What every app of the org answers, written down once. Each answer has a schema and a sample. An app and its readers test against the same files, so a change that breaks a reader is caught early.

The rules and the layout of the folders are in `constitution/tech-stack.md`; the purpose and the words are in `constitution/mission.md`.

## What is here

- `envelope/v1/`, `error/v1/`, `event/v1/`: the shapes every answer, error and event shares.
- `envelope/v2/`: the envelope with `source` for where an answer comes from and each error a Problem (`problem/v1`). An app moves to it with its next contract version; envelope/v1 stays until each app moves, and `version/v1` stays on envelope/v1 for good.
- `envelope/v3/`: envelope/v2 with `problems`, what an app worked around while still giving a good answer, such as an answer converted down to an older contract version (`converted_down`, ADR-S16), which an error would wrongly mark as failed. A contract on envelope/v1 or v2 is never a target for converting down.
- `problem/v1/`: the one Problem shape every app reports what went wrong in (ADR-S21), for errors and for problems an answer worked around. `error/v1` stays until each app moves.
- `version/v1/`: the one version answer every app serves in every contract version (ADR-S03): the contract versions it serves, in `supportedContracts`. designer/v1's Version stays until each app moves.
- `version/v2/`: the version answer from version/v2 on, on envelope/v3: the contract versions an app reads (`reads`: its own and each older one its down steps reach) and writes (`writes`: its own and the one before). version/v1 stays, made by the down step, for readers that ask for it.
- `mankit/v1/`: what mankit serve answers for every app: the record a detached `serve` writes, the `servers` answers, the `<product>.stream.gapped` event a client of `GET /api/events` gets when it missed events, the change queue's ticket, backend marker, the `<app> queue` answer and the `<app> config queue` answer (`QueueSettings`), the store's journal (`StoreJournal` and `StoreJournalChange`, JSON lines; how its lines follow each other and how long it lives is in `store-journal.schema.json`) and its unfinished-change mark (`StoreUnfinished`), the `<app> store unfinished list` and `clear` answers (`StoreUnfinishedList`, `StoreUnfinishedCleared`), the `<app> license show` answer (`LicenseShow`: the license state, the build, the licenses taken and refused, and each feature's state and where it is unlocked from), the signed license file (`license-file.schema.json`, its payload in `license-payload.schema.json`), the `<app> help -o json` answer (`Help`: the commands and flags the H-1 layout shows, and the old forms of renamed commands), and in `defs.schema.json` the shapes mankit gives every app's answers, such as a feature switch's state, which each app's own feature list references, a store key (`storeKey`) and a change cut short (`storeUnfinishedChange`). Each app lists that event in its own `events.asyncapi.yaml`.
- `<app>/<version>/`: one app's contract, named by its product ID. `designer/v1/` is the first.
- `control-room/v1/`: control-room's answers, on envelope/v2: the snapshot (`Snapshot`), its settings (`SettingList`), the outlaws (`OutlawList`) and the work list (`WorkList`), with the shapes they share in `defs.schema.json`. Its other kinds are in the envelope only for now (forms.yaml).
- `upgrade.go`: the steps between versions (CT-RULE-3), each converting both ways in one place (ADR-S16): `Upgrade<Kind>…` makes the newer form, such as `UpgradeEnvelope` from envelope/v1 to envelope/v2 and `UpgradeEnvelopeToV3`, and `Downgrade<Kind>To<v>` the older one, saying as paths (`problems.0`, `data.writes`) what the older form could not hold. Each is tested by pairs in `testdata/upgrade/<dir>/<from>-<to>/`. Go only for now, as the readers that mix versions are Go.
- `apiversion/v1/forms.yaml`: the one table of every apiVersion form. A row names a current form, the old forms it replaces and its kinds. A reader that meets an old form finds the row whose `old` list holds it, and gets the current form and the upgrade command (`hubman migrate <app> <folder>`).

An answer has five fields: `apiVersion`, `kind`, `ran`, `data` and `errors`. `ran` says where it came from: the name, the app, its version and the contract version. A reader asks for a version with `--contract v1` or the `Accept-Contract: v1` header. An app that cannot answer it gives the error `unsupported_contract` and lists the versions it supports. In envelope/v2, `ran` is `source`, and that error is a Problem of type `unsupported_contract`; the app's version answer lists the versions it serves.

An event is a CloudEvents 1.0 event. An app lists its events in `events.asyncapi.yaml` (AsyncAPI 3.0), and each event has a schema and a sample.

## The change queue on disk

Every app that changes a served thing shares one queue in `<served folder>/<app folder>/.queue/` (ADR-S14), and two app versions may share it at once. So within `mankit/v1` these rules never change; a change to them is a new version.

- The folder and its `bad/` are mode 0700. It holds `lock`, `turn`, `turn.lock`, `bad/`, `backend`, `.gitignore` (holding `*`), the tickets, `<id>.json` (`queue-ticket.schema.json`), and beside each ticket its lock file, `<id>.lock`. Names starting with a dot are not tickets and are left alone. Among them may be mankit store's marks of an unfinished change, `.unfinished-<id>.json` (`store-unfinished.schema.json`). A lock file is never moved to `bad/`: one without its ticket is removed once no process holds it. `lock` and `turn.lock` are never removed. Anything else that is not a readable ticket is moved to `bad/`, never over a file already there, and said.
- A ticket and the `backend` marker both carry apiVersion `elderlygate/mankit/queue/v1`, told apart by kind: `QueueTicket` and `QueueBackend`.
- `backend`, when present, names the backend every writer of the folder takes turns with (`queue-backend.schema.json`): `files` is the rest of this section; without it the backend is `files`. It never holds an address or a token, and it is changed only while holding `lock` with the line empty. A backend that is unknown, cannot be read or cannot be reached is refused with `queue_backend_unsupported`; an app never falls back to files.
- A lock is an exclusive lock on a whole lock file (`flock` on Linux and macOS, `LockFileEx` on Windows), only ever tried without waiting. Only `lock`, `turn.lock` and `<id>.lock` are locked; a file others read (`turn`, a ticket) is never locked, since a lock on Windows keeps others from reading what it covers.
- `lock` is held while a ticket is written, the folder is read, or a ticket is removed. A process locks `<id>.lock` and then writes its ticket whole (through a temporary dot-file renamed into place), both before `lock` is given up.
- `<id>.lock` is opened and removed only while holding `lock`, but for one case: a process that cannot get `lock` while leaving removes its own ticket and `<id>.lock` without it, since ids are never reused. A process holds its ticket's `<id>.lock` from before writing the ticket to after removing it. A reader that can take that lock knows the process has ended and removes the ticket and its lock file.
- A process takes the turn, by locking `turn.lock`, only while it holds `lock` and its ticket is first, and holds it until its change is written and read back. While it holds it, `turn` holds its ticket's id and the time it took the turn (RFC 3339), one per line, written whole; it removes `turn` before giving the turn up.
- A process limits its own change: past its limit (2 minutes unless its app sets `queue.limit`) its write is told to stop; a write that then stops with an error fails with `change_too_long`, and the turn is given up once the write has returned. A write that returned no error is always read back and reported as done or `not_read_back`, never `change_too_long`. Others never judge how long a holder runs. How long a process waits for its turn (`queue.wait`, 30 seconds unless set) bounds only its own waiting. So apps sharing a queue may set both apart.
- A ticket is touched every 5 seconds while it waits and runs. A ticket whose lock is held but which was not touched for 30 seconds is stale and moved to `bad/`. These two never change, as every app reads every other's tickets by them.
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
- every file in `testdata/good/` meets its schema: the shapes a schema takes beyond its one sample;
- every pair in `testdata/upgrade/<dir>/<from>-<to>/` (`<case>.<from>.json` and `<case>.<to>.json`) is a sample of the version the step reads and the sample it makes of it, what a down step leaves out is `<case>.lost.json` (none means nothing), no file is without its partner, and every `<case>.refused.json` is refused by the step;
- the apiVersion table lists each form once, and each old form in one row only; a row with old forms has its upgrade command.
