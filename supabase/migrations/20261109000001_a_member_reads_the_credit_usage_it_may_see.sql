-- member.credits_used is not readable through the member table, so the
-- settings screen reads usage through this function instead. An administrator
-- sees every member of the company and whether each one takes a billed seat;
-- anybody else sees their own row only, so one colleague's spending stays
-- between them and the administrators.

create function public.company_credit_usage()
returns table (member_id uuid, credits_used bigint, takes_a_seat boolean)
language sql
stable
security definer
set search_path = ''
as $$
  select m.id, m.credits_used, public.member_takes_a_seat(m.kind, m.status)
  from public.member m
  where m.company_id = internal.company_of_member(public.my_member())
    and (public.is_company_admin() or m.id = public.my_member());
$$;

revoke execute on function public.company_credit_usage() from public, anon;
grant execute on function public.company_credit_usage() to authenticated, service_role;
