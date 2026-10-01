# Database migrations

`<YYYYMMDDHHMMSS>_<snake_case>.sql`, applied in lexical order by
`swappy-backend migrate` (also on server start). Each file runs in its own
transaction and is recorded in `schema_migrations`.

Never edit a migration that is already committed — add a new one.
