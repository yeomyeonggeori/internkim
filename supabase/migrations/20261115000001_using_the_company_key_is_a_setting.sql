-- Who pays for a company's model calls is one rule: the company's own
-- OpenRouter key pays when the company's computer holds one and
-- company.uses_own_model_key is on, and credits pay through the
-- model-gateway edge function otherwise. The two never mix. Holding a key and
-- using it are separate: turning the setting off keeps the key where it is.
--
-- A computer decides once, when its services start. It tells the gateway
-- whether it holds a key and is answered whether the company uses it, and
-- each computer keeps what it said and what it was answered, so the setup
-- page shows what is running on the computer that serves the company, and a
-- setting changed since then waits for that computer's next start.
--
-- Credits can pay while the company has credits of either kind left. That is
-- the gateway's own refusal, so it is written once, here, and the refusal and
-- the web app both ask it.
--
-- Existing companies keep who pays them today: one whose credits can pay or
-- whose subscription is active or trialing spent credits first, so it starts
-- with the setting off; every other company paid with its key whenever it
-- held one, so it starts with it on. A key entered from now on turns the
-- setting on, because entering a key is the intent to use it.

alter table public.company
  add column uses_own_model_key boolean not null default true;

comment on column public.company.uses_own_model_key is
  'whether the company''s own OpenRouter key pays for its model calls when its computer holds one; credits pay through the model gateway otherwise';

alter table public.box
  add column holds_model_key boolean,
  add column uses_own_model_key_at_start boolean,
  add column model_key_reported_at timestamptz;

alter table public.agent
  add column holds_model_key boolean,
  add column uses_own_model_key_at_start boolean,
  add column model_key_reported_at timestamptz;

comment on column public.box.holds_model_key is
  'whether this box held an OpenRouter key when its services last started; null until it has said';
comment on column public.box.uses_own_model_key_at_start is
  'the company''s uses_own_model_key this box was answered when its services last started';
comment on column public.agent.holds_model_key is
  'whether the computer holding this agent key held an OpenRouter key when its services last started; null until it has said';
comment on column public.agent.uses_own_model_key_at_start is
  'the company''s uses_own_model_key that computer was answered when its services last started';

create function public.company_credits_can_pay(target_company uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select coalesce(
    (select subscription_credits + permanent_credits > 0 from public.company where id = target_company),
    false
  );
$$;

revoke all on function public.company_credits_can_pay(uuid) from public, anon, authenticated;
grant execute on function public.company_credits_can_pay(uuid) to service_role;

comment on function public.company_credits_can_pay(uuid) is
  'whether the model gateway would let the company spend credits: it has credits of either kind left';

create or replace function public.company_member_credit_refusal(
  target_company uuid,
  target_member uuid,
  default_usage_limit bigint
)
returns text
language plpgsql
stable
security definer
set search_path = ''
as $$
declare
  usage_limit bigint;
  used bigint;
begin
  select coalesce((rules -> 'creditPolicy' ->> 'usageLimit')::bigint, default_usage_limit)
  into usage_limit
  from public.company
  where id = target_company;
  if not found then
    return 'no_company';
  end if;

  if not public.company_credits_can_pay(target_company) then
    return 'no_credits';
  end if;
  if target_member is null then
    return null;
  end if;

  select credits_used into used
  from public.member
  where id = target_member and company_id = target_company;
  if not found then
    return 'not_a_member';
  end if;
  if used >= usage_limit then
    return 'usage_limit';
  end if;
  return null;
end;
$$;

create function public.keep_model_key_report(
  reporting_company uuid,
  reporting_box text,
  reporting_agent_key_hash text,
  holds boolean
)
returns boolean
language plpgsql
security definer
set search_path = ''
as $$
declare
  answered boolean;
begin
  select uses_own_model_key into answered from public.company where id = reporting_company;
  if not found then
    return null;
  end if;
  update public.box
    set holds_model_key = holds, uses_own_model_key_at_start = answered, model_key_reported_at = now()
    where company_id = reporting_company and public_key = reporting_box;
  update public.agent
    set holds_model_key = holds, uses_own_model_key_at_start = answered, model_key_reported_at = now()
    where company_id = reporting_company and api_key_hash = reporting_agent_key_hash;
  return answered;
end;
$$;

revoke all on function public.keep_model_key_report(uuid, text, text, boolean) from public, anon, authenticated;
grant execute on function public.keep_model_key_report(uuid, text, text, boolean) to service_role;

comment on function public.keep_model_key_report(uuid, text, text, boolean) is
  'keeps what a company computer said when its services started, whether it holds an OpenRouter key, on its box or its agent key, and answers whether the company uses its own key, which it keeps beside it';

create function public.company_model_key_report(target_company uuid)
returns table (holds_model_key boolean, uses_own_model_key_at_start boolean)
language sql
stable
security definer
set search_path = ''
as $$
  select reported.holds_model_key, reported.uses_own_model_key_at_start
  from (
    select serving.holds_model_key, serving.uses_own_model_key_at_start, serving.model_key_reported_at
    from public.box as serving
    where serving.company_id = target_company
      and serving.role = 'service'
      and not public.company_served_by_connection_file(target_company)
    union all
    select standing.holds_model_key, standing.uses_own_model_key_at_start, standing.model_key_reported_at
    from public.agent as standing
    where standing.company_id = target_company
      and standing.revoked_at is null
  ) as reported
  where reported.model_key_reported_at is not null
  order by reported.model_key_reported_at desc
  limit 1;
$$;

revoke all on function public.company_model_key_report(uuid) from public, anon, authenticated;
grant execute on function public.company_model_key_report(uuid) to service_role;

comment on function public.company_model_key_report(uuid) is
  'what the computer serving the company said when its services last started: whether it held a key and whether it was told to use it. The service box, or a computer on a connection file when one serves the company; no row until that computer has said';

update public.company
set uses_own_model_key = false
where subscription_status in ('active', 'trialing')
   or public.company_credits_can_pay(id);
