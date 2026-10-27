alter table public.member add column preferences jsonb not null default '{}'::jsonb;

create function internal.channel_order_of(held jsonb)
returns jsonb
language sql
immutable
set search_path = ''
as $$
  select jsonb_build_object(
    'pinnedChannels', coalesce(held -> 'pinnedChannels', '[]'::jsonb),
    'channelOrder', coalesce(held -> 'channelOrder', '[]'::jsonb)
  );
$$;

create function internal.pinned_channels_reordered(held jsonb, asked text[])
returns jsonb
language sql
immutable
strict
set search_path = ''
as $$
  select coalesce(jsonb_agg(kept.channel order by asked_place.place nulls last, kept.place), '[]'::jsonb)
  from jsonb_array_elements_text(held) with ordinality as kept(channel, place)
  left join lateral (
    select min(listed.place) as place
    from unnest(asked) with ordinality as listed(channel, place)
    where listed.channel = kept.channel
  ) as asked_place on true;
$$;

create function internal.channels_in_order(listed text[])
returns jsonb
language sql
immutable
strict
set search_path = ''
as $$
  select coalesce(jsonb_agg(first_seen.channel order by first_seen.place), '[]'::jsonb)
  from (
    select named.channel, min(named.place) as place
    from unnest(listed) with ordinality as named(channel, place)
    where named.channel <> ''
    group by named.channel
  ) as first_seen;
$$;

revoke execute on function internal.channel_order_of(jsonb)
  from public, anon, authenticated, service_role;
revoke execute on function internal.pinned_channels_reordered(jsonb, text[])
  from public, anon, authenticated, service_role;
revoke execute on function internal.channels_in_order(text[])
  from public, anon, authenticated, service_role;

create function public.channel_order_get()
returns jsonb
language sql
stable
security definer
set search_path = ''
as $$
  select internal.channel_order_of(member.preferences)
  from public.member
  where member.id = public.my_member();
$$;

create function public.channel_pin(channel text)
returns jsonb
language sql
security definer
set search_path = ''
as $$
  update public.member
  set preferences = case
    when member.preferences -> 'pinnedChannels' ? channel then member.preferences
    else jsonb_set(
      member.preferences,
      '{pinnedChannels}',
      coalesce(member.preferences -> 'pinnedChannels', '[]'::jsonb) || to_jsonb(channel)
    )
  end
  where member.id = public.my_member()
  returning internal.channel_order_of(member.preferences);
$$;

create function public.channel_unpin(channel text)
returns jsonb
language sql
security definer
set search_path = ''
as $$
  update public.member
  set preferences = jsonb_set(
    member.preferences,
    '{pinnedChannels}',
    coalesce(member.preferences -> 'pinnedChannels', '[]'::jsonb) - channel
  )
  where member.id = public.my_member()
  returning internal.channel_order_of(member.preferences);
$$;

create function public.channel_order_set(pinned text[], ordered text[])
returns jsonb
language sql
security definer
set search_path = ''
as $$
  update public.member
  set preferences = member.preferences || jsonb_strip_nulls(jsonb_build_object(
    'pinnedChannels', internal.pinned_channels_reordered(member.preferences -> 'pinnedChannels', pinned),
    'channelOrder', internal.channels_in_order(ordered)
  ))
  where member.id = public.my_member()
  returning internal.channel_order_of(member.preferences);
$$;

revoke execute on function public.channel_order_get() from public, anon;
revoke execute on function public.channel_pin(text) from public, anon;
revoke execute on function public.channel_unpin(text) from public, anon;
revoke execute on function public.channel_order_set(text[], text[]) from public, anon;

grant execute on function public.channel_order_get() to authenticated, service_role;
grant execute on function public.channel_pin(text) to authenticated, service_role;
grant execute on function public.channel_unpin(text) to authenticated, service_role;
grant execute on function public.channel_order_set(text[], text[]) to authenticated, service_role;
