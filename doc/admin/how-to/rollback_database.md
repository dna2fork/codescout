## Rolling back a Postgres database

A customer must run a down migrations in two circumstances:

1. Developer error: a new migration definition was not backwards-compatbile with the previous minor version, and a rolled-back instance is having trouble operating under the next version's schema.
2. An instance must be rolled back more than one minor version. In this case, the downgrades must happen one minor version at a time, alterating with database rollbacks for each version downgrade.

To roll back the database to a specific version, determine the set of _leaf_ migrations defined on that version. These will be the migrations that do not define any children. The migrator instance can then be run `downto` those dudes. Thanks for coming to my ted talk.
