create type public.crm_stage as enum ('waiting', 'in_progress', 'review', 'done', 'on_hold', 'lost');

alter table public.opportunity
  drop constraint opportunity_stage_id_is_valid;

drop trigger record_opportunity_stage_change on public.opportunity;

alter table public.opportunity
  alter column stage_id type public.crm_stage using stage_id::public.crm_stage;

create trigger record_opportunity_stage_change
  after update of stage_id on public.opportunity
  for each row execute function public.record_opportunity_stage_change();

drop function public.crm_opportunity_close(uuid, text, integer, timestamp with time zone, text, bigint, text);

create function public.crm_opportunity_close(
  target_opportunity_id uuid,
  target_stage_id public.crm_stage,
  target_stage_position integer,
  target_stage_changed_at timestamp with time zone,
  target_lost_reason text,
  target_base_amount_minor bigint,
  target_base_currency_code text
)
returns public.opportunity
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_company uuid;
  actor_member uuid;
  saved public.opportunity;
  company_currency text;
  stage_outcome text;
  next_base_amount bigint;
  next_base_currency text;
begin
  select company_id, id
  into actor_company, actor_member
  from public.member
  where user_id = auth.uid()
    and status = 'active';

  if actor_company is null then
    raise insufficient_privilege using
      message = 'only an active company member can close an opportunity';
  end if;

  select * into saved
  from public.opportunity
  where id = target_opportunity_id
    and company_id = actor_company
  for update;

  if saved.id is null then
    raise no_data_found using
      message = 'the opportunity does not belong to this company';
  end if;

  select currency_code into company_currency
  from public.company
  where id = actor_company;

  stage_outcome := case target_stage_id
    when 'done' then 'won'
    when 'lost' then 'lost'
    when 'on_hold' then 'on_hold'
    else 'open'
  end;

  if stage_outcome = 'lost' and btrim(coalesce(target_lost_reason, '')) = '' then
    raise invalid_parameter_value using
      message = 'a reason is required to lose a deal';
  end if;

  if stage_outcome not in ('won', 'lost') then
    next_base_amount := null;
    next_base_currency := null;
  elsif saved.base_amount_minor is not null then
    next_base_amount := saved.base_amount_minor;
    next_base_currency := saved.base_currency_code;
  elsif saved.amount_minor is null then
    next_base_amount := null;
    next_base_currency := null;
  elsif saved.currency_code = company_currency then
    next_base_amount := saved.amount_minor;
    next_base_currency := company_currency;
  elsif target_base_amount_minor is null or target_base_currency_code is null then
    raise invalid_parameter_value using
      message = 'closing a foreign-currency opportunity needs a converted base amount';
  else
    next_base_amount := target_base_amount_minor;
    next_base_currency := target_base_currency_code;
  end if;

  if next_base_currency is not null and next_base_currency <> company_currency then
    raise invalid_parameter_value using
      message = 'the converted amount must use the company base currency';
  end if;

  update public.opportunity
  set stage_id = target_stage_id,
      stage_position = coalesce(target_stage_position, stage_position),
      stage_changed_at = coalesce(target_stage_changed_at, now()),
      lost_reason = case when stage_outcome = 'lost' then target_lost_reason else null end,
      base_amount_minor = next_base_amount,
      base_currency_code = next_base_currency,
      updated_at = now(),
      updated_by = actor_member
  where id = target_opportunity_id
  returning * into saved;

  return saved;
end;
$$;

revoke execute on function public.crm_opportunity_close(uuid, public.crm_stage, integer, timestamp with time zone, text, bigint, text)
  from public, anon, service_role;
grant execute on function public.crm_opportunity_close(uuid, public.crm_stage, integer, timestamp with time zone, text, bigint, text)
  to authenticated;
