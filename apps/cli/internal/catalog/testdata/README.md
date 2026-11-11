# Registry contract fixtures

These are recorded responses from the production registry
(`https://registry.devex.sh`) on 2026-11-11:

- `applications.json`: `GET /api/v1/applications?limit=2&page=1`, with the
  pagination rewritten to a single page (`total` 2, `hasNext` false) and a
  fixed `meta.timestamp`.
- `application.json`: `GET /api/v1/applications/git`.

Platform objects use the YAML catalog's snake_case keys (`install_method`,
`install_command`, `platform_requirements`, ...). Older seeds wrote camelCase;
the client accepts both. Re-record both files when the registry routes change,
and update the contract expectations in `client_test.go` with them.
