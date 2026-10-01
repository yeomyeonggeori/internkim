create or replace function public.my_member()
returns uuid
language sql
security definer
stable
set search_path = public
as $$
  select id
  from public.member
  where user_id = auth.uid()
    and status not in ('departed', 'withdrawn');
$$;
