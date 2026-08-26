create table public.notification (
  member_id uuid not null references public.member on delete cascade,
  conversation_id text not null,
  is_muted boolean not null,
  updated_at timestamptz not null default now(),
  primary key (member_id, conversation_id)
);

insert into public.notification (member_id, conversation_id, is_muted, updated_at)
select member_id, conversation_id, true, muted_at from public.muted_conversation;

drop function public.conversation_mute(text);
drop function public.conversation_unmute(text);
drop table public.muted_conversation;

alter table public.notification enable row level security;

create policy notification_kept_by_owner on public.notification
  for all to authenticated
  using (member_id = public.my_member())
  with check (member_id = public.my_member());

grant select on public.notification to authenticated;
grant all on public.notification to service_role;

create function public.conversation_mute(conversation text)
returns void
language sql
security definer
set search_path = public
as $$
  insert into public.notification (member_id, conversation_id, is_muted)
  values (public.my_member(), conversation, true)
  on conflict (member_id, conversation_id)
  do update set is_muted = true, updated_at = now();
$$;

create function public.conversation_unmute(conversation text)
returns void
language sql
security definer
set search_path = public
as $$
  insert into public.notification (member_id, conversation_id, is_muted)
  values (public.my_member(), conversation, false)
  on conflict (member_id, conversation_id)
  do update set is_muted = false, updated_at = now();
$$;
