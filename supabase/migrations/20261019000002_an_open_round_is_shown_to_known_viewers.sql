create function internal.data_room_viewer_is_known(target_company uuid)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (select 1 from public.data_room_link_session where id = auth.uid())
    or public.data_room_member(target_company) is not null
    or exists (select 1 from public.data_room_share share
      where share.company_id = target_company and share.audience = 'email' and public.data_room_share_applies(share));
$$;

create function internal.data_room_without_open_rounds(shown jsonb)
returns jsonb language sql immutable set search_path = ''
as $$
  select jsonb_set(shown, '{records}', coalesce((
    select jsonb_agg(record order by position)
    from jsonb_array_elements(shown -> 'records') with ordinality as entry(record, position)
    where record ->> 'status' is distinct from 'in_progress'), '[]'::jsonb));
$$;

create or replace function public.data_room_company(target_company uuid)
returns jsonb language sql stable security definer set search_path = ''
as $$
  select case when internal.data_room_viewer_is_known(target_company) then shown
    else internal.data_room_without_open_rounds(shown) end
  from (select internal.data_room_company_shown(target_company, array(
    select code from public.data_room_category
    where company_id = target_company and public.data_room_may_read(target_company, code))) as shown) company_view;
$$;

revoke all on function internal.data_room_viewer_is_known(uuid),
  internal.data_room_without_open_rounds(jsonb) from public, anon, authenticated;
