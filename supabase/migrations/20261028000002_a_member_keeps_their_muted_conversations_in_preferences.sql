update public.member
set preferences = jsonb_set(member.preferences, '{mutedChannels}', muted.conversations)
from (
  select notification.member_id,
    jsonb_agg(notification.conversation_id order by notification.updated_at) as conversations
  from public.notification
  where notification.is_muted
  group by notification.member_id
) as muted
where muted.member_id = member.id;

create function public.my_muted_conversations()
returns text[]
language sql
stable
security definer
set search_path = ''
as $$
  select array(
    select jsonb_array_elements_text(member.preferences -> 'mutedChannels')
    from public.member
    where member.id = public.my_member()
  );
$$;

revoke execute on function public.my_muted_conversations() from public, anon;
grant execute on function public.my_muted_conversations() to authenticated, service_role;

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

  insert into public.notification (member_id, conversation_id, is_muted)
  values (public.my_member(), conversation, true)
  on conflict (member_id, conversation_id)
  do update set is_muted = true, updated_at = now();
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

  insert into public.notification (member_id, conversation_id, is_muted)
  values (public.my_member(), conversation, false)
  on conflict (member_id, conversation_id)
  do update set is_muted = false, updated_at = now();
$$;
