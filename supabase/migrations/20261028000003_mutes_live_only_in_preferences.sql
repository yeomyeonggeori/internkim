create or replace function public.conversation_mute(conversation text)
returns void
language sql
security definer
set search_path = ''
as $$
  update public.member
  set preferences = case
    when member.preferences -> 'mutedChannels' ? conversation then member.preferences
    else jsonb_set(
      member.preferences,
      '{mutedChannels}',
      coalesce(member.preferences -> 'mutedChannels', '[]'::jsonb) || to_jsonb(conversation)
    )
  end
  where member.id = public.my_member();
$$;

create or replace function public.conversation_unmute(conversation text)
returns void
language sql
security definer
set search_path = ''
as $$
  update public.member
  set preferences = jsonb_set(
    member.preferences,
    '{mutedChannels}',
    coalesce(member.preferences -> 'mutedChannels', '[]'::jsonb) - conversation
  )
  where member.id = public.my_member();
$$;

do $$
begin
  if exists (
    select 1
    from public.notification
    join public.member on member.id = notification.member_id
    where notification.is_muted
      and not coalesce(member.preferences -> 'mutedChannels' ? notification.conversation_id, false)
  ) then
    raise exception 'a member muted a conversation that preferences.mutedChannels lacks; copy it there before dropping public.notification';
  end if;
end $$;

drop table public.notification;
