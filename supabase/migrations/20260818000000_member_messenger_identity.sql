-- A member's messenger account is part of who they are here, so it belongs on
-- the member rather than in a table beside it. contact held one row per member
-- per platform, carrying the member id, the external id, and the messenger's
-- own spelling of a name the member row already has.
--
-- What is left in contact is what it is for: people the company deals with who
-- are not members of it.

alter table public.member add column messenger jsonb not null default '{}'::jsonb;

-- One account per messenger, named by a non-empty string. A check constraint
-- cannot hold a subquery, so the shape is decided by a function it can call.
create or replace function public.is_messenger_accounts(accounts jsonb)
returns boolean
language sql
immutable
as $$
  select jsonb_typeof(accounts) = 'object'
     and not exists (
       select 1
       from jsonb_each(accounts) as entry
       where jsonb_typeof(entry.value) <> 'string' or entry.value #>> '{}' = ''
     );
$$;

alter table public.member add constraint member_messenger_is_platform_to_account
  check (public.is_messenger_accounts(messenger));

update public.member
set messenger = accounts.held
from (
  select member_id, jsonb_object_agg(platform, external_id) as held
  from public.contact
  where member_id is not null
  group by member_id
) as accounts
where accounts.member_id = public.member.id;

delete from public.contact where member_id is not null;

alter table public.contact drop column member_id;

-- contact becomes member's sibling: one row per person the company deals with,
-- holding their accounts the same way a member holds theirs. Keyed by platform
-- and account id, the same person reached on two messengers was two contacts
-- and there was nowhere to say they were one.
alter table public.contact add column id uuid not null default gen_random_uuid();
alter table public.contact add column messenger jsonb not null default '{}'::jsonb;
alter table public.contact add constraint contact_messenger_is_platform_to_account
  check (public.is_messenger_accounts(messenger));

update public.contact set messenger = jsonb_build_object(platform, external_id);

do $$
declare
  current_key text;
begin
  select constraint_name into current_key
  from information_schema.table_constraints
  where table_schema = 'public' and table_name = 'contact' and constraint_type = 'PRIMARY KEY';
  if current_key is not null then
    execute format('alter table public.contact drop constraint %I', current_key);
  end if;
end $$;

alter table public.contact add primary key (id);
alter table public.contact drop column platform;
alter table public.contact drop column external_id;
