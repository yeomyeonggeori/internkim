create function public.company_credits_refunded(
  target_company uuid,
  target_event text,
  refunded_kind text,
  refunded_credits bigint
)
returns bigint
language plpgsql
security definer
set search_path = ''
as $$
declare
  balance bigint;
  taken bigint;
begin
  if refunded_kind not in ('subscription', 'permanent') then
    raise exception 'credits of kind % cannot be refunded', refunded_kind
      using errcode = '22023';
  end if;
  if exists (select 1 from public.company_credit_entry where stripe_event_id = target_event) then
    return 0;
  end if;

  select case refunded_kind when 'subscription' then subscription_credits else permanent_credits end
  into balance
  from public.company
  where id = target_company
  for update;
  if not found then
    raise exception 'no company % for Stripe event %', target_company, target_event
      using errcode = 'P0002';
  end if;

  taken := least(greatest(refunded_credits, 0), balance);
  if taken > 0 then
    insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits, stripe_event_id)
    values (target_company, refunded_kind, 'refund', -taken, target_event);
  end if;
  return taken;
end;
$$;

create or replace function public.company_subscription_changed(
  target_company uuid,
  target_customer text,
  target_subscription text,
  target_status text,
  status_at timestamptz
)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  left_over bigint;
begin
  if not exists (select 1 from public.company where id = target_company) then
    raise exception 'no company % for Stripe subscription %', target_company, target_subscription
      using errcode = 'P0002';
  end if;

  update public.company
  set stripe_customer_id = target_customer,
      stripe_subscription_id = target_subscription,
      subscription_status = target_status,
      subscription_status_at = status_at
  where id = target_company
    and (subscription_status_at is null or subscription_status_at <= status_at)
  returning subscription_credits into left_over;

  if target_status = 'canceled' and left_over > 0 then
    insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits)
    values (target_company, 'subscription', 'expiry', -left_over);
  end if;
end;
$$;

revoke execute on function public.company_credits_refunded(uuid, text, text, bigint)
  from public, anon, authenticated;
grant execute on function public.company_credits_refunded(uuid, text, text, bigint)
  to service_role;
