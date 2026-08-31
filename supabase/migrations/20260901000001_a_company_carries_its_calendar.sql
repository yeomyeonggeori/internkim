alter table public.company add column calendar jsonb not null default '{}';

comment on column public.company.calendar is
  'what the company keeps about its calendar as a whole: the subscription address, and later the calendars it is connected to';

create function public.calendar_subscription_save(target_token_hash text)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_admin boolean;
  written jsonb;
begin
  select company_id, is_admin
  into actor_company, actor_admin
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null or not coalesce(actor_admin, false) then
    raise insufficient_privilege using
      message = 'the calendar belongs to the company, and only an active company admin registers its subscription';
  end if;

  if target_token_hash is null then
    written := (select calendar - 'subscription' from public.company where id = actor_company for update);
  elsif target_token_hash !~ '^[0-9a-f]{64}$' then
    raise invalid_parameter_value using
      message = 'a subscription is kept as what its address hashes to';
  else
    written := (
      select jsonb_set(calendar, '{subscription}', jsonb_build_object('tokenHash', target_token_hash), true)
      from public.company where id = actor_company for update
    );
  end if;

  update public.company set calendar = written where id = actor_company;
end;
$$;

revoke execute on function public.calendar_subscription_save(text) from public, anon, service_role;
grant execute on function public.calendar_subscription_save(text) to authenticated;
