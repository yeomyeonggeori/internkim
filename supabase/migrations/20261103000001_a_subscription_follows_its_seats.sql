create function public.member_takes_a_seat(member_kind public.member_kind, member_status public.member_status)
returns boolean
language sql
immutable
as $$
  select member_kind = 'human' and member_status not in ('departed', 'withdrawn');
$$;

create function public.company_billed_seats(target_company uuid)
returns integer
language sql
stable
security definer
set search_path = public
as $$
  select count(*)::integer from public.member
  where company_id = target_company and public.member_takes_a_seat(kind, status);
$$;

revoke execute on function public.company_billed_seats(uuid) from public, anon, authenticated;
grant execute on function public.company_billed_seats(uuid) to service_role;

create function public.ask_the_subscription_to_follow_its_seats()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
declare
  seats_before boolean := tg_op <> 'INSERT' and public.member_takes_a_seat(old.kind, old.status);
  seats_after boolean := tg_op <> 'DELETE' and public.member_takes_a_seat(new.kind, new.status);
  target_company uuid := case when tg_op = 'DELETE' then old.company_id else new.company_id end;
  app_url text;
  job_secret text;
begin
  if seats_before = seats_after then
    return null;
  end if;
  if not exists (select 1 from public.company where id = target_company and stripe_subscription_id is not null) then
    return null;
  end if;

  select decrypted_secret into app_url from vault.decrypted_secrets where name = 'day_digest_app_url';
  select decrypted_secret into job_secret from vault.decrypted_secrets where name = 'scheduled_job_secret';
  if coalesce(app_url, '') = '' or job_secret is null then
    raise notice 'company % changed its seats, but day_digest_app_url or scheduled_job_secret is unset', target_company;
    return null;
  end if;

  perform net.http_post(
    url := rtrim(app_url, '/') || '/api/stripe/seats',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || job_secret,
      'Content-Type', 'application/json'
    ),
    body := jsonb_build_object('company_id', target_company),
    timeout_milliseconds := 30000
  );
  return null;
end;
$$;

create trigger a_subscription_follows_its_seats
after insert or update of kind, status or delete on public.member
for each row execute function public.ask_the_subscription_to_follow_its_seats();
