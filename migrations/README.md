# Postgres Migrations

The children of this directory contain migrations for each Postgres database instance:

- `frontend` is the main database (things should go here unless there is a good reason)
- `codeintel` is a database containing only processed LSIF data (which can become extremely large)
- `codeinsights` is a TimescaleDB database, containing only Code Insights time series data

The migration path for each database instance is the same and is described below. Each of the database instances described here are deployed separately, but are designed to be _overlayable_ to reduce friction during development. That is, we assume that the names in each database do not overlap so that the same connection parameters can be used for both database instances. Each database also has a uniquely named schema versions table:

| database       | schema version table name        |
| -------------- | -------------------------------- |
| `frontend`     | `schema_migrations`              |
| `codeintel`    | `codeintel_schema_migrations`    |
| `codeinsights` | `codeinsights_schema_migrations` |

## Migrating up and down

Up migrations will happen automatically in development on service startup. In production environments, they are run by the [`migrator`](../cmd/migrator) instance. You can run migrations manually during development via `sg`:

- `sg migration up` runs all migrations to the latest version
- `sg migration up -db=frontend -target=<version>` runs up migrations (relative to the current database version) on the frontend database until it hits the target version
- `sg migration down -db=codeintel` runs one _down_ migration (relative to the current database version) on the codeintel database

## Adding a migration

**IMPORTANT:** All migrations must be backwards-compatible, meaning that _existing_ code must be able to operate successfully against the _new_ (post-migration) database schema. Consult [_Writing database migrations_](https://docs.sourcegraph.com/dev/background-information/sql/migrations.md) in our developer documentation for additional context.

To create a new migration file, run the following command.

```
$ sg migration add -db=<db_name> <my_migration_name>
Migration files created
 Up migration: ~/migrations/frontend/1528395961_my_migration_name.up.sql
 Down migration: ~/migrations/frontend/1528395961_my_migration_name.down.sql
```

This will create an _up_ and _down_ pair of migration files (printed by the given command). Add SQL statements to these files that will perform the desired migration. After adding SQL statements to those files, update the schema doc via `go generate ./internal/database/` (or regenerate everything via `./dev/generate.sh`).

To pass CI, you'll additionally need to:

- Ensure that your new migrations run against the current Go unit tests
- Ensure that your new migrations can be run up, then down, then up again (idempotency test)
- Ensure that your new migrations do not break the Go unit tests published with the previous release (backwards-compatibility test)

### Reverting a migration

If a PR which contains a DB migration was reverted, it may still have been applied to Sourcegraph.com, k8s.sgdev.org, etc. due to their rollout schedules. In some cases, it may also have been part of a Sourcegraph release.

To fix this, you should create a PR to revert the migration from the DB. Say the migration files were:

- `1234_do_something.up.sql`
- `1234_do_something.down.sql`

You should then:

1. Rename the files to `1234_reverted.up.sql` and `1234_reverted.down.sql`
2. Replace the contents of those files with just:

```sql
BEGIN;

-- This migration was reverted, see: <github issue link>

COMMIT;
```

3. Add a new migration using `./dev/db/add_migration.sh <database> undo_something` which will consume the next sequential migration ID.
4. Your new `.up.sql` migration should contain the contents of the old `1234_reverted.down.sql` and you will need to update the migration to run down migrations idempotently, i.e. using `IF EXISTS` etc. everywhere as _some instances running it may not have run the up migration_.
5. Your new `.down.sql` should be an empty migration.

For an example of how this looks: https://github.com/sourcegraph/sourcegraph/pull/25717
