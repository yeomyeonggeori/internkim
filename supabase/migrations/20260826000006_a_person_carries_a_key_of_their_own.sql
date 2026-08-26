-- A key said which company was calling. Nothing said who, so anything reached
-- through one was the company itself and no person in it.
--
-- A key that names a member is that member calling. It reaches exactly what
-- they reach signed in, because it is turned into their own session and row
-- level security decides the rest. A key that names nobody is a device, which
-- is what every key was until now.
alter table public.agent
  add column member_id uuid references public.member on delete cascade;

comment on column public.agent.member_id is
  'the member this key speaks for; null for a device key, which speaks for the company';

-- A device's name is unique in its company, and a person's is unique to them,
-- so two people may both call a key "laptop".
alter table public.agent drop constraint agent_company_id_name_key;

create unique index agent_device_name on public.agent (company_id, name)
  where member_id is null;

create unique index agent_member_name on public.agent (company_id, member_id, name)
  where member_id is not null;

create index agent_member on public.agent (member_id) where member_id is not null;
