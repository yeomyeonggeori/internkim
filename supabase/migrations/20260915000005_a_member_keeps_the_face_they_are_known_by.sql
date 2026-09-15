drop function public.member_picture_keep_drawn(text);

create function public.member_picture_keep(picture_path text)
returns boolean
language plpgsql
security definer
set search_path = public
as $$
declare
  own uuid := public.my_member();
begin
  if own is null then
    raise exception 'a kept picture belongs to someone who is signed in'
      using errcode = '42501';
  end if;
  if public.asset_company(picture_path) is distinct from internal.company_of_member(own)
    or public.asset_scope(picture_path) is distinct from 'shared'
    or public.asset_segment(picture_path, 3) is distinct from 'member' then
    raise exception 'a kept picture lives in the company shared member folder, not at %', picture_path
      using errcode = '22023';
  end if;
  update public.member
    set profile_image = picture_path
    where member.id = own
      and member.profile_image is distinct from picture_path
      and (member.profile_image is null or public.asset_segment(member.profile_image, 3) = 'member');
  return found;
end;
$$;

revoke execute on function public.member_picture_keep(text) from public, anon;
grant execute on function public.member_picture_keep(text) to authenticated, service_role;
