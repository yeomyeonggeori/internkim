-- A member who is in every channel is told about every channel. Muting is per
-- conversation and per member, and it is a row only when muted: the default is
-- that a conversation notifies, so an empty table is a company that hears
-- everything.
create table public.muted_conversation (
  member_id uuid not null references public.member on delete cascade,
  conversation_id text not null,
  muted_at timestamptz not null default now(),
  primary key (member_id, conversation_id)
);

alter table public.muted_conversation enable row level security;

create policy muted_conversation_kept_by_owner on public.muted_conversation
  for all to authenticated
  using (member_id = public.my_member())
  with check (member_id = public.my_member());

grant select, insert, delete on public.muted_conversation to authenticated;
grant all on public.muted_conversation to service_role;

create function public.mute_conversation(conversation text)
returns void
language sql
security definer
set search_path = public
as $$
  insert into public.muted_conversation (member_id, conversation_id)
  values (public.my_member(), conversation)
  on conflict (member_id, conversation_id) do nothing;
$$;

create function public.unmute_conversation(conversation text)
returns void
language sql
security definer
set search_path = public
as $$
  delete from public.muted_conversation
  where member_id = public.my_member() and conversation_id = conversation;
$$;
