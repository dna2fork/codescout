## Rolling back a Postgres database

If a customer downgrades their instance to a previous version, they may a need to to downgrade their database schema in two circumstances:

1. On deveroper error. If a migration added in the successor version that was not backwards-compatible, the code of the previous version may struggle operating with the new schema. This may be an emergency situation in which the previous schema must be restored to bring the instance back to a stable state.
2. If the customer is downgrading more than one minor version, each downgrade will need to alternate with a symmetric downgrade of the database. 

To roll back the database to a specific version, we need to determine the set of _leaf_ migrations defined at that version. This can be done via the `sg` development tool.

```bash
$ sg migration leaves v3.36.0

Leaf migrations for "frontend" defined at commit "v3.36.0"
 1528395968: (track fork namespace on spec)

Leaf migrations for "codeintel" defined at commit "v3.36.0"
 1000000030: (lsif data implementations)

Leaf migrations for "codeinsights" defined at commit "v3.36.0"
 1000000025: (captured values series points)
```

With these targets, we can downgrade the database schema down to the previous version by invoking the `migrator` and giving it a specific migration target.

```bash
export SOURCEGRAPH_VERSION="The version you are downgrading from"
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
    downto \
    -db=frontend \
    -target=1528395968
```

This command should be run for each of the schemas listed in the output of the `sg` command.
