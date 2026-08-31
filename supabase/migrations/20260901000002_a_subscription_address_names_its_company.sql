create function public.calendar_subscription_company(target_token_hash text)
returns uuid
language sql
security definer
set search_path = ''
stable
as $$
  select id from public.company
  where target_token_hash ~ '^[0-9a-f]{64}$'
    and calendar #>> '{subscription,tokenHash}' = target_token_hash;
$$;

revoke execute on function public.calendar_subscription_company(text) from public, anon, authenticated;
grant execute on function public.calendar_subscription_company(text) to service_role;
