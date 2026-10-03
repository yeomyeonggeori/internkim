alter table public.data_room_role_category
  drop constraint data_room_role_category_company_id_role_code_fkey,
  add constraint data_room_role_category_company_id_role_code_fkey foreign key (company_id, role_code)
    references public.data_room_role on update cascade on delete cascade;
alter table public.data_room_share
  drop constraint data_room_share_company_id_role_code_fkey,
  add constraint data_room_share_company_id_role_code_fkey foreign key (company_id, role_code)
    references public.data_room_role on update cascade on delete cascade;
alter table public.data_room_link
  drop constraint data_room_link_company_id_role_code_fkey,
  add constraint data_room_link_company_id_role_code_fkey foreign key (company_id, role_code)
    references public.data_room_role on update cascade on delete cascade;

update public.data_room_role role
set code = renamed.new_code, name = renamed.name, name_ko = renamed.name_ko
from (values ('employee', 'member', 'Member', '구성원'), ('people', 'human-resources', 'HR', '인사'))
  as renamed (old_code, new_code, name, name_ko)
where role.code = renamed.old_code
  and not exists (select 1 from public.data_room_role taken
    where taken.company_id = role.company_id and taken.code = renamed.new_code);

update public.data_room_role set name_ko = '재무' where code = 'finance' and name_ko = '재무 담당';
update public.data_room_role set name_ko = '회계·세무 자문' where code = 'accountant' and name_ko = '회계사·세무사';

create or replace function internal.seed_member_data_room()
returns trigger language plpgsql security definer set search_path = ''
as $$
begin
  insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
  values (new.company_id, 'member', 'member', new.id, true);
  if new.is_admin then
    insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
    values (new.company_id, 'leadership', 'member', new.id, true);
  end if;
  return new;
end;
$$;

create temporary table circle_role on commit drop as
select circle.id as circle_id, circle.company_id, circle.name,
  case lower(btrim(circle.name))
    when 'c-level' then 'leadership'
    when 'representative' then 'leadership'
    when 'hr' then 'human-resources'
    when 'staff' then 'member'
    when 'admin' then null
    else lower(btrim(circle.name))
  end as role_code
from public.circle circle;

insert into public.data_room_role (company_id, code, name)
select company_id, role_code, name from circle_role
where role_code ~ '^[a-z][a-z0-9-]*$'
on conflict (company_id, code) do nothing;

insert into public.data_room_share (company_id, role_code, audience, member_id, can_download)
select distinct circle_role.company_id, circle_role.role_code, 'member', held.member_id, true
from circle_role
join public.circle_member held on held.circle_id = circle_role.circle_id
join public.data_room_role role on role.company_id = circle_role.company_id and role.code = circle_role.role_code
where not exists (
  select 1 from public.data_room_share share
  where share.company_id = circle_role.company_id and share.role_code = circle_role.role_code
    and share.audience = 'member' and share.member_id = held.member_id and share.revoked_at is null
);
