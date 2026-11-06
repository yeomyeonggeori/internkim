create table public.company_credit_top_up_setting (
  company_id uuid primary key references public.company on delete cascade,
  enabled boolean not null default false,
  threshold_credits bigint not null check (threshold_credits > 0),
  amount bigint not null check (amount > 0),
  updated_at timestamptz not null default now()
);

comment on table public.company_credit_top_up_setting is
  'whether the company buys credits by itself, below which balance, and for how much';
comment on column public.company_credit_top_up_setting.amount is
  'what one top-up charges, in the smallest unit of the subscription''s currency';

create table public.company_credit_top_up (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  status text not null default 'pending' check (status in ('pending', 'paid', 'failed')),
  stripe_invoice_id text unique,
  failure text,
  created_at timestamptz not null default now(),
  settled_at timestamptz,
  constraint company_credit_top_up_settled_has_a_time
    check ((status = 'pending') = (settled_at is null)),
  constraint company_credit_top_up_failure_names_a_reason
    check ((status = 'failed') = (failure is not null))
);

create unique index company_credit_top_up_one_pending_idx
  on public.company_credit_top_up (company_id) where status = 'pending';
create index company_credit_top_up_company_id_created_at_idx
  on public.company_credit_top_up (company_id, created_at);

comment on table public.company_credit_top_up is
  'one charge for credits the company asked for by itself, pending until Stripe pays or declines it';

alter table public.company_credit_top_up_setting enable row level security;
alter table public.company_credit_top_up enable row level security;

create policy company_credit_top_up_setting_read_by_admin on public.company_credit_top_up_setting
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin());

create policy company_credit_top_up_read_by_admin on public.company_credit_top_up
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin());

revoke all on public.company_credit_top_up_setting from anon, authenticated, service_role;
revoke all on public.company_credit_top_up from anon, authenticated, service_role;
grant select on public.company_credit_top_up_setting to authenticated, service_role;
grant select on public.company_credit_top_up to authenticated, service_role;

create function internal.top_ups_per_month()
returns integer
language sql
immutable
as $$
  select 5;
$$;

create function internal.top_up_settles_within()
returns interval
language sql
immutable
as $$
  select interval '1 hour';
$$;

create function public.company_credit_top_up_configured(
  target_company uuid,
  is_enabled boolean,
  threshold bigint,
  charged_amount bigint
)
returns void
language sql
security definer
set search_path = ''
as $$
  insert into public.company_credit_top_up_setting (company_id, enabled, threshold_credits, amount)
  values (target_company, is_enabled, threshold, charged_amount)
  on conflict (company_id) do update
  set enabled = excluded.enabled,
      threshold_credits = excluded.threshold_credits,
      amount = excluded.amount,
      updated_at = now();
$$;

create function internal.company_credit_top_up_asked()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
  company_row public.company;
  stale_top_up uuid;
  top_up uuid;
  app_url text;
  job_secret text;
begin
  if new.entry_kind <> 'usage' then
    return null;
  end if;
  select * into company_row from public.company where id = new.company_id;
  if company_row.stripe_subscription_id is null or company_row.subscription_status not in ('active', 'trialing') then
    return null;
  end if;
  if not exists (
    select 1 from public.company_credit_top_up_setting
    where company_id = new.company_id and enabled
      and company_row.subscription_credits + company_row.permanent_credits < threshold_credits
  ) then
    return null;
  end if;

  select id into stale_top_up from public.company_credit_top_up
  where company_id = new.company_id and status = 'pending' and created_at < now() - internal.top_up_settles_within();
  if stale_top_up is not null then
    perform public.company_credit_top_up_failed(stale_top_up, null, 'the charge did not settle in time');
    return null;
  end if;

  if (select count(*) from public.company_credit_top_up
      where company_id = new.company_id and created_at > now() - interval '30 days') >= internal.top_ups_per_month() then
    return null;
  end if;

  insert into public.company_credit_top_up (company_id)
  values (new.company_id)
  on conflict (company_id) where status = 'pending' do nothing
  returning id into top_up;
  if top_up is null then
    return null;
  end if;

  select decrypted_secret into app_url from vault.decrypted_secrets where name = 'day_digest_app_url';
  select decrypted_secret into job_secret from vault.decrypted_secrets where name = 'scheduled_job_secret';
  if coalesce(app_url, '') = '' or job_secret is null then
    raise notice 'company % asked for a top-up, but day_digest_app_url or scheduled_job_secret is unset', new.company_id;
    return null;
  end if;

  perform net.http_post(
    url := rtrim(app_url, '/') || '/api/stripe/top-up',
    headers := jsonb_build_object(
      'Authorization', 'Bearer ' || job_secret,
      'Content-Type', 'application/json'
    ),
    body := jsonb_build_object('top_up_id', top_up),
    timeout_milliseconds := 30000
  );
  return null;
end;
$$;

create trigger company_credit_usage_asks_for_a_top_up
  after insert on public.company_credit_entry
  for each row execute function internal.company_credit_top_up_asked();

create function public.company_credit_top_up_paid(
  target_top_up uuid,
  target_event text,
  target_invoice text,
  granted_credits bigint
)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  target_company uuid;
begin
  if exists (select 1 from public.company_credit_entry where stripe_event_id = target_event) then
    return;
  end if;

  update public.company_credit_top_up
  set status = 'paid', stripe_invoice_id = target_invoice, failure = null, settled_at = now()
  where id = target_top_up and status <> 'paid'
  returning company_id into target_company;
  if target_company is null then
    return;
  end if;

  insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits, stripe_event_id)
  values (target_company, 'permanent', 'purchase', granted_credits, target_event);
end;
$$;

create function public.company_credit_top_up_failed(
  target_top_up uuid,
  target_invoice text,
  reason text
)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  target_company uuid;
begin
  update public.company_credit_top_up
  set status = 'failed', stripe_invoice_id = coalesce(target_invoice, stripe_invoice_id), failure = reason, settled_at = now()
  where id = target_top_up and status = 'pending'
  returning company_id into target_company;
  if target_company is null then
    return;
  end if;

  update public.company_credit_top_up_setting
  set enabled = false, updated_at = now()
  where company_id = target_company;
end;
$$;

revoke execute on function internal.top_ups_per_month() from public, anon, authenticated;
revoke execute on function internal.top_up_settles_within() from public, anon, authenticated;
revoke execute on function internal.company_credit_top_up_asked() from public, anon, authenticated;
revoke execute on function public.company_credit_top_up_configured(uuid, boolean, bigint, bigint)
  from public, anon, authenticated;
grant execute on function public.company_credit_top_up_configured(uuid, boolean, bigint, bigint) to service_role;
revoke execute on function public.company_credit_top_up_paid(uuid, text, text, bigint) from public, anon, authenticated;
grant execute on function public.company_credit_top_up_paid(uuid, text, text, bigint) to service_role;
revoke execute on function public.company_credit_top_up_failed(uuid, text, text) from public, anon, authenticated;
grant execute on function public.company_credit_top_up_failed(uuid, text, text) to service_role;
