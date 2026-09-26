create function public.account_of_address(address text)
returns uuid
language sql
stable
security definer
set search_path = ''
as $$
  select account.id
  from auth.users account
  where lower(account.email) = lower(address)
  limit 1;
$$;

revoke execute on function public.account_of_address(text) from public, anon, authenticated;
grant execute on function public.account_of_address(text) to service_role;
