create or replace function public.conversation_mute_for(muted_member uuid, conversation text)
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
  where member.id = muted_member;
$$;

revoke execute on function public.conversation_mute_for(uuid, text) from public, anon, authenticated;
grant execute on function public.conversation_mute_for(uuid, text) to service_role;

create or replace function public.conversation_mute(conversation text)
returns void
language sql
security definer
set search_path = ''
as $$
  select public.conversation_mute_for(public.my_member(), conversation);
$$;
