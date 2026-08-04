create table public.agent (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  name text not null,
  api_key_hash text not null unique,
  created_at timestamptz not null default now(),
  last_seen_at timestamptz,
  revoked_at timestamptz,
  unique (company_id, name)
);

create index on public.agent (company_id) where revoked_at is null;

insert into public.agent (company_id, name, api_key_hash)
select id, 'first', host_secret_hash from public.company where host_secret_hash is not null;

alter table public.company drop column host_secret_hash;

alter table public.agent enable row level security;

