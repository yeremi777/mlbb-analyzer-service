# 0001 — Internal package layout

## Status

Accepted.

## Context

One Go module carries two workloads against one database: an HTTP API that
serves reads per request, and a collector plus seeder that write in batches on a
schedule. They have opposite failure semantics — a request that half-succeeds is
a bug, a collection run that half-succeeds is a success — but they share hero
identity, matchup shapes, and the same Postgres instance.

Without a stated rule, every new file poses the same two questions: which
package owns this, and does it need an interface. Answered ad hoc, the two
workloads grow parallel stacks that drift.

## Decision

### Rings

Packages sit in one of four rings. A package may import its own ring's
dependencies downward only.

| Ring | Named after | Packages | May import |
| --- | --- | --- | --- |
| Domain | the business | `domain` | nothing internal |
| Adapter | the external system it speaks to | `postgres`, `dataset`, `upstream/*` | domain |
| Orchestrator | the job it performs | `api`, `collector`, `analyzer` | domain, adapters |
| Wiring | the binary | `cmd/*` | everything |

`internal/domain` holds the types and the invariants that hold regardless of
where data came from. It performs no I/O and imports no other internal package.

Adapter packages are named for the boundary they own, using whichever noun is
clearest: the technology when the service speaks a standard protocol
(`postgres`), the vendor when it speaks that vendor's own protocol (`moonton`),
the content when neither is (`dataset`). The name tells a reader what breaks
when the thing on the far side changes.

Every package name says what the package provides, never what it is about. At
most one package may carry a proper noun, and only an adapter may carry it — the
one that talks to that proper noun's system. This is why the domain package is
`domain` and not `mlbb`: naming it for the game put a second brand name beside
`moonton`, and neither name then said which was the model and which was the
network client.

A directory groups packages only when two or more siblings are the same kind of
thing. Nesting creates no relationship in Go — the parent gets no privileged
access and the import name is the last path segment alone — so a parent
directory earns its place as a category, never as structure. `upstream` holds
the feeds this service reads but does not control: they change shape without
warning, they answer over a network, and they fail transiently. `postgres` and
`dataset` stay flat because we own what they carry.

Orchestrator packages are named for the job: `api` serves, `collector` ingests,
`analyzer` scores. They are siblings. The serving path and the ingest path
diverge here and nowhere earlier — there is no backend package and no data
package.

`cmd/*` reads configuration, constructs adapters, and hands them to the code
that does the work. It writes no SQL, no retry loop, and no request pacing: a
job needing any of those lives in an orchestrator package. A job needing none of
them — one fetch, one write — stays in `cmd` rather than growing a package for
the sake of symmetry.

One binary per deployable unit. Jobs that share wiring become its subcommands
rather than separate binaries: `collector patches`, `collector stats` and
`collector all` load the same environment and open the same connection, and one
cron entry runs the lot.

### Packages

| Package | Ring | Owns | Depends on |
| --- | --- | --- | --- |
| `domain` | domain | Hero, Matchup, HeroMatchup, Proof, HeroRankStat, and the invariants that hold within and across a whole set | nothing |
| `config` | — | reading the environment | nothing |
| `upstream/moonton` | adapter | the rank-statistics endpoint: its URL, request body, and wire shapes | nothing |
| `upstream/liquipedia` | adapter | the patch calendar page: its query, and the HTML it returns | `domain` |
| `dataset` | adapter | the authored JSON files on disk: their layout, names, and decoding | `domain` |
| `postgres` | adapter | every SQL statement, and the transaction surfaces callers use | `domain`, `upstream/moonton` |
| `ratelimit` | adapter | per-client request quota over a fixed window | nothing |
| `collector` | orchestrator | one daily ingest run: retry, pacing, skip, partial failure | `upstream/moonton` |
| `analyzer` | orchestrator | scoring and explaining a matchup through an AI provider chain | `domain` |
| `api` | orchestrator | the public HTTP surface | `domain`, `postgres`, `analyzer`, `ratelimit` |

`domain` sits under everything and reaches for nothing. The `upstream` feeds,
`config` and `ratelimit` are self-contained because they describe a system, not
the game.

The three orchestrators never import each other. `api` is the whole serving
path; `collector` is the whole ingest path; `analyzer` is called by `api` alone.
A change to how heroes are stored touches `postgres`; a change to how they are
authored touches `dataset`; a change to what makes a hero valid touches `domain`
and is caught in every path at once.

### Persistence: functions by default, interfaces by need

A query is an exported function taking a context, a connection, and domain
arguments:

```go
func ListHeroes(ctx context.Context, q Querier) ([]domain.Hero, error)
func SyncHeroes(ctx context.Context, tx pgx.Tx, heroes []domain.Hero) error
```

Reads take `Querier`, the read-only surface shared by pool, connection, and
transaction. Writes take `pgx.Tx`: the caller owns the transaction boundary and
therefore owns what a partial failure means. An adapter that must open its own
transaction takes `Executor`, which is `Querier` plus `Begin`.

There is no repository struct and no interface enumerating every query.

An interface appears only where an orchestrator has branching logic worth
testing without its dependency — retry, backoff, partial failure, skip
conditions. That interface is declared in the consuming package, named for the
role it plays there, and lists only the methods that consumer calls:

```go
// package collector
type Store interface {
    HasSnapshotDate(ctx context.Context, date string) (bool, error)
    InsertCombo(ctx context.Context, records []moonton.Record, combo moonton.Combo, date string) (int, error)
}
```

The adapter package satisfies it with a thin struct over the plain functions.
The functions stay the primary surface; the struct is an adaptor, not a
replacement.

### Verbs carry the workload

The exported verb tells a reader which path a function belongs to:

- `Get`, `List` — serving path, read-only, `Querier`.
- `Sync` — seed path, declarative full replace against `public.*`, `pgx.Tx`.
- `Insert` — ingest path, append-only against `raw.*`, `pgx.Tx`.

### Wire types and domain types

A writer accepts an adapter's own type when that type *is* the upstream shape,
and a domain type otherwise. The test is what the type carries, not which schema
it lands in.

`moonton.Record` is the upstream shape: it carries `Raw json.RawMessage`, the
hero's JSON object exactly as the endpoint sent it, which
`raw.hero_rank_snapshots.payload` stores unmodified. A domain type in front of
it would exist only to mirror a wire format, so `InsertSnapshots` takes
`moonton.Record` and `postgres` imports `upstream/moonton`.

`liquipedia` has no such type. Its upstream shape is an HTML document, and
`Patch` is what a regex extracted from a table cell — an interpretation, not a
recording. Nothing would be mirrored by naming it in the domain, so
`FetchPatches` returns `domain.Patch` and `postgres` does not import
`upstream/liquipedia`.

Both write to `raw.*`. The schema they target did not decide either case.

## Consequences

The compiler enforces most of the layout: `domain` cannot grow an I/O dependency
without a visible import, and an orchestrator cannot reach into another
orchestrator.

Two workloads against one schema stay separated by transaction ownership rather
than by duplicated packages. Adding a third consumer — a staging transform, an
export job — means one new orchestrator package and no change to rings one or
two.

The cost is that a genuinely shared piece of orchestration has nowhere to live;
it must move down into an adapter or the domain, or be duplicated. Duplication
is the preferred answer until a third caller appears.
