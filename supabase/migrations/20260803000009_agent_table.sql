-- A company may run more than one agent — a spare box, or the new one alongside the
-- old during a move — and replacing one should not cut the other off mid-sentence.
-- A column could hold only one key and rotating it killed the running agent, so the
-- key moves to a row that can be added, retired and seen.
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

-- Carry over whatever was already issued, so an agent that is running keeps working.
insert into public.agent (company_id, name, api_key_hash)
select id, 'first', host_secret_hash from public.company where host_secret_hash is not null;

alter table public.company drop column host_secret_hash;

alter table public.agent enable row level security;

-- No policy: only the control plane touches this. Colleagues have no business
-- reading key hashes, and an agent never reads this table at all.
