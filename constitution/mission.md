# Mission

## Why

Apps answer people, scripts, AI agents and each other. One place says what each answer looks like, so an app and its readers agree, and a change to an answer is caught before it breaks a reader.

## What a user gets

- **CT-MISSION-1** 2026-11-11 · For each app and version, the shape of every answer it gives, as a schema.
- **CT-MISSION-2** 2026-11-11 · Sample answers that the app and its readers both test against.
- **CT-MISSION-3** 2026-11-11 · The steps that upgrade an answer from one version to the next.

## Who uses it

- **CT-MISSION-4** 2026-11-11 · People building an app: its tests check its answers against these schemas.
- **CT-MISSION-5** 2026-11-11 · Readers of an app (control-room, other apps, scripts, AI agents): they read its answers by these schemas.

## Words

| Word | Meaning |
|---|---|
| Contract | What one app answers at one version: its kinds, their schemas and samples. |
| Envelope | The outer shape every answer shares: `apiVersion`, `kind`, `data`, `errors`. |
| Kind | One type of answer, such as a list of blocks. |
| Schema | The rules an answer of one kind must meet. |
| Sample | A real answer kept as a file, which both sides test against. |
