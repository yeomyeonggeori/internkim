-- Before the agent can act as the member who spoke, it has to prove it is this
-- company's host. The secret is issued once at install and only its hash is kept,
-- so a copy of this table is not a set of working credentials.
alter table public.company add column host_secret_hash text;
