alter table public.member add column name text;

update public.member
  set name = nullif(trim(account.raw_user_meta_data ->> 'full_name'), '')
  from auth.users account
  where account.id = public.member.user_id and public.member.name is null;

create or replace function public.bind_member_to_new_user()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  update public.member
    set user_id = new.id,
        name = coalesce(name, nullif(trim(new.raw_user_meta_data ->> 'full_name'), ''))
    where email = new.email and user_id is null;
  return new;
end;
$$;
