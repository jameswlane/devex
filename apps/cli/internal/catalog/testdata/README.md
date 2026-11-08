# Registry contract fixtures

These fixtures are derived offline, not downloaded from production.
`applications.json` follows `app/api/v1/applications/route.ts`,
`lib/transformation-service.ts` (`transformApplications`), and
`lib/response-optimization.ts` (`createPaginatedResponse`). Values follow the
Git YAML used by `scripts/sync-database.ts`, with the camelCase platform
representation consumed by `scripts/seed.ts` (missing booleans/arrays default to
false/[]).
`application.json` records the unwrapped name-addressed detail response's fields
plus database fields from `prisma/schema.prisma`; IDs and times are deterministic
fixture values.
Update the fixture and contract expectations together when the route changes.
