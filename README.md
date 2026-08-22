# pg-genrole

## Permission Matrix

Per database, `pg-genrole` creates `{database}.read` and `{database}.readwrite` roles and grants:

| Object                               | Privilege                          | `.read` | `.readwrite` |
| ------------------------------------ | ---------------------------------- | ------- | ------------ |
| Database                             | CONNECT                            | ✅      | ✅           |
| Schema                               | USAGE                              | ✅      | ✅           |
| Tables (existing)                    | SELECT                             | ✅      | ✅           |
| Tables (existing)                    | INSERT, UPDATE, DELETE, TRUNCATE   | ❌      | ✅           |
| Tables (new, via default privileges) | SELECT                             | ✅      | ✅           |
| Tables (new, via default privileges) | INSERT, UPDATE, DELETE, TRUNCATE   | ❌      | ✅           |
| Views                                | SELECT                             | ✅      | ✅           |
| Materialized views                   | SELECT                             | ✅      | ✅           |
| Materialized views                   | OWNER (incl. ALTER, REFRESH, DROP) | ❌      | ✅           |
| Sequences                            | USAGE, SELECT                      | ✅      | ✅           |
| Functions                            | EXECUTE                            | ✅      | ✅           |
| DDL modification (CREATE, ALTER, DROP) | ❌                                | ❌      | ❌           |

> ⚠️ Neither role can run DDL — `USAGE` on schemas is granted but never `CREATE`, so both roles are denied CREATE/ALTER/DROP. Exception: `.readwrite` takes ownership of materialized views, so it can refresh, alter, or drop *those*. Views only get SELECT.

## E2E Guide

```bash
docker compose -f e2e/compose.yml up
go test -tags e2e ./e2e/...
```

- Cleanup

```bash
docker compose -f e2e/compose.yml down
```
