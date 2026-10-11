create function internal.circle_reads_category(target_company uuid, target_circle text, target_category text)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.circle_category permission
    join public.data_room_category category on category.company_id = permission.company_id and category.code = target_category
    where permission.company_id = target_company and permission.circle_id = target_circle
      and permission.category_code in (category.code, category.parent)
  );
$$;

revoke all on function internal.circle_reads_category(uuid, text, text) from public, anon, authenticated;

create or replace function internal.data_room_link_may_read(target_company uuid, target_category text, download boolean)
returns boolean language sql stable security definer set search_path = ''
as $$
  select exists (
    select 1 from public.data_room_link_session session
    join public.data_room_link link on link.id = session.link_id
    where session.id = auth.uid() and session.expires_at > now()
      and link.company_id = target_company and link.revoked_at is null and link.expires_at > now()
      and (not download or link.can_download)
      and internal.circle_reads_category(target_company, link.circle_id, target_category)
      and internal.data_room_member_can_read(target_company, link.creator_member_id, target_category, download)
  );
$$;

create function public.data_room_circle_categories(target_company uuid, target_circle text)
returns text[] language sql stable security definer set search_path = ''
as $$
  select case when internal.circle_fits_member(target_company, public.data_room_member(target_company), target_circle, false)
    then coalesce(array_agg(category.code order by category.code)
      filter (where internal.circle_reads_category(target_company, target_circle, category.code)), array[]::text[])
  end
  from public.data_room_category category
  where category.company_id = target_company;
$$;

revoke all on function public.data_room_circle_categories(uuid, text) from public, anon;
grant execute on function public.data_room_circle_categories(uuid, text) to authenticated;
