# How to troubleshoot a dirty database

This document will take you through how to resolve a 'dirty database' error. During an upgrade, the `pgsql`, `codeintel-db`, and `codeinsights-db` databases must be migrated. If the upgrade was interrupted during the migration, this can result in a 'dirty database' error.

The error will look something like this:

```
INFO[02-08|00:40:55] Checked current version                  schema=frontend appliedVersions="[1528395834 1528395835 1528395836 ... 1528395969 1528395970 1528395971]" pendingVersions=[1528395947] failedVersions=[]
error: 1 error occurred:
	* dirty database: schema "frontend" marked the following migrations as failed: 1528395947

The target schema is marked as dirty and no other migration operation is seen running on this schema. The last migration operation over this schema has failed (or, at least, the migrator instance issuing that migration has died). Please contact support@sourcegraph.com for further assistance.
```

Resolving this error requires manually attempting to run the migrations that are marked as pending or failed.

## Prerequisites

* This document assumes that you are installing Sourcegraph or were attempting an upgrade when an error occurred. 
* **NOTE: If you encountered this error during an upgrade, ensure you followed the [proper step upgrade process documented here.](https://docs.sourcegraph.com/admin/updates) If you skipped a minor version during an upgrade, you will need to revert back to the last minor version your instance was on before following the steps in this document.**

The following procedure requires that you are able to execute commands from inside the database container. Learn more about shelling into [kubernetes](https://docs.sourcegraph.com/admin/install/kubernetes/operations#access-the-database), [docker-compose](https://docs.sourcegraph.com/admin/install/docker-compose/operations#access-the-database), and [Sourcegraph single-container](https://docs.sourcegraph.com/admin/install/docker/operations#access-the-database) instances at these links. 

## Steps to resolve

### 1. Identify incomplete migration

When migrations run, the `migration_logs` table is updated. Before each migration attempt, a new row is inserted indicating the migration version and direction and the start time. Once the migration is complete (or fails), the row is updated with the finished time and message with details about any error that occurred.

A failed migration may have explicitly failed due to a SQL/environment error, or it may fail if the migrator instance dies before updating its log entry. In this case, the validation mechanism that runs on app startup will wait for running migrators to complete their current work. Pending _but inactive_ migration attempts will be correct reported as failed, as seen again below.

```
INFO[02-08|00:40:55] Checked current version                  schema=frontend appliedVersions="[1528395834 1528395835 1528395836 ... 1528395969 1528395970 1528395971]" pendingVersions=[1528395947] failedVersions=[]
error: 1 error occurred:
	* dirty database: schema "frontend" marked the following migrations as failed: 1528395947

The target schema is marked as dirty and no other migration operation is seen running on this schema. The last migration operation over this schema has failed (or, at least, the migrator instance issuing that migration has died). Please contact support@sourcegraph.com for further assistance.
```

In this example, we need to re-apply the migration `1528395947`. **We'll note this number for use in step 2.**

The `migration_logs` table can also be queried directly. The following query gives an overview of the most recent migration attempts broken down by version.

```sql
WITH ranked_migration_logs AS (
	SELECT
		migration_logs.*,
		ROW_NUMBER() OVER (PARTITION BY schema, version ORDER BY finished_at DESC) AS row_number
	FROM migration_logs
)
SELECT *
FROM ranked_migration_logs
WHERE row_number = 1
ORDER BY version
```

### 2. Run the sql queries to finish incomplete migrations

Migration definitions for each database schema can be found in the children of the [`migrations/` directory](https://github.com/sourcegraph/sourcegraph/tree/main/migrations).

1. **Find the target migration with the version number identified in [step 1](#1-identify-incomplete-migration)**.

2. **Run the code from the identified `<version>/up.sql` file explicitly using the `psql` CLI:**
   * It’s possible that one or more commands from the migration ran successfully already. In these cases you may need to run the sql transaction in pieces. For example if a migration file creates multiple indexes and one index already exists you'll need to manually run this transaction skipping that line or adding `IF NOT EXISTS` to the transaction.
   * If you’re running into unique index creation errors because of duplicate values please let us know at support@sourcegraph.com or via your enterprise support channel.
   * There may be other error cases that don't have an easy admin-only resolution, in these cases please let us know at support@sourcegraph.com or via your enterprise support channel.

### 3. Add a migration log entry

1. **Ensure the migration applied, then signal that the migration has been run.**
   * Run the `migrator` instance against your database to create an explicit migration log.

    ```bash
    export SOURCEGRAPH_VERSION="The version you are upgrading to"
    docker run --rm --name migrator_$SOURCEGRAPH_VERSION \
        -e PGHOST='pgsql' \
        -e PGPORT='5432' \
        -e PGUSER='sg' \
        -e PGPASSWORD='sg' \
        -e PGDATABASE='sg' \
        -e PGSSLMODE='disable' \
        -e CODEINTEL_PGHOST='codeintel-db' \
        -e CODEINTEL_PGPORT='5432' \
        -e CODEINTEL_PGUSER='sg' \
        -e CODEINTEL_PGPASSWORD='sg' \
        -e CODEINTEL_PGDATABASE='sg' \
        -e CODEINTEL_PGSSLMODE='disable' \
        -e CODEINSIGHTS_PGHOST='codeinsights-db' \
        -e CODEINSIGHTS_PGPORT='5432' \
        -e CODEINSIGHTS_PGUSER='postgres' \
        -e CODEINSIGHTS_PGPASSWORD='password' \
        -e CODEINSIGHTS_PGDATABASE='postgres' \
        -e CODEINSIGHTS_PGSSLMODE='disable' \
        --network=docker-compose_sourcegraph \
        sourcegraph/migrator:$SOURCEGRAPH_VERSION \
        add-log \
        -db=frontend \
        -version=1528395968
    ```


   * **Do not mark the migration table as clean if you have not verified that the migration was successfully completed.**
   * Checking to see if a migration ran successfully requires looking at the migration’s `sql` file, and verifying that `sql` queries contained in the migration file have been applied to tables in the database. 
   * _Note: Many migrations do nothing but create tables and/or indexes or alter them._
   * You can get a description of a table and its associated indexes quickly using the `\d <table name>` `psql` shell command (note lack of semicolon). Using this information, you can determine whether a table exists, what columns it contains, and what indexes on it exist. Use this information to determine if commands in a migration ran successfully before adding a migration log entry.

2. **Start Sourcegraph again and the remaining migrations should succeed, otherwise repeat this procedure again starting from the [Identify incomplete migration](#1-identify-incomplete-migration) step.**

## Further resources

* [Sourcegraph - Upgrading Sourcegraph to a new version](https://docs.sourcegraph.com/admin/updates)
* [Migrations README.md](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/README.md) (Note some of the info contained here pertains to running Sourcegraphs development environment and should not be used on production instances)
