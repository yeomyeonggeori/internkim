-- Stripe tells the web app when an invoice is paid and when a subscription
-- changes. These are the two writes those events make, each in one
-- transaction, so a retried event and a half-applied one are the same thing:
-- nothing changes the second time.
--
-- An invoice that opens a period (the first one, or a renewal) expires what is
-- left of the subscription credits, grants the new period's credits and moves
-- subscription_ends_at forward, which returns every member's usage to zero. An
-- invoice inside a period, a seat added partway, only grants. The grant line
-- carries the Stripe event, and that event is what makes the call happen once.

create function public.company_subscription_invoice_paid(
  target_company uuid,
  target_event text,
  target_customer text,
  target_subscription text,
  granted_credits bigint,
  opens_period boolean,
  period_ends_at timestamptz
)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  left_over bigint;
begin
  if exists (select 1 from public.company_credit_entry where stripe_event_id = target_event) then
    return;
  end if;

  update public.company
  set stripe_customer_id = target_customer,
      stripe_subscription_id = target_subscription
  where id = target_company;
  if not found then
    raise exception 'no company % for Stripe event %', target_company, target_event
      using errcode = 'P0002';
  end if;

  if opens_period then
    select subscription_credits into left_over from public.company where id = target_company for update;
    if left_over > 0 then
      insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits)
      values (target_company, 'subscription', 'expiry', -left_over);
    end if;
  end if;

  if granted_credits > 0 then
    insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits, stripe_event_id)
    values (target_company, 'subscription', 'grant', granted_credits, target_event);
  end if;

  if opens_period then
    update public.company
    set subscription_ends_at = period_ends_at
    where id = target_company
      and (subscription_ends_at is null or subscription_ends_at < period_ends_at);
  end if;
end;
$$;

alter table public.company add column subscription_status_at timestamptz;

comment on column public.company.subscription_status_at is
  'when Stripe created the event subscription_status came from; an older event arriving later leaves the status alone';

create function public.company_subscription_changed(
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
    and (subscription_status_at is null or subscription_status_at <= status_at);
end;
$$;

create or replace function internal.company_billing_is_written_by_the_server()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  if current_user not in ('anon', 'authenticated') then
    return new;
  end if;
  if (new.stripe_customer_id, new.stripe_subscription_id, new.subscription_status, new.subscription_status_at,
      new.subscription_ends_at, new.subscription_credits, new.permanent_credits)
    is distinct from
     (case when tg_op = 'UPDATE' then
        (old.stripe_customer_id, old.stripe_subscription_id, old.subscription_status, old.subscription_status_at,
         old.subscription_ends_at, old.subscription_credits, old.permanent_credits)
      else (null::text, null::text, null::text, null::timestamptz, null::timestamptz, 0::bigint, 0::bigint) end) then
    raise exception 'a company''s subscription and credits are written by the server'
      using errcode = '42501';
  end if;
  return new;
end;
$$;

revoke execute on function public.company_subscription_invoice_paid(uuid, text, text, text, bigint, boolean, timestamptz)
  from public, anon, authenticated;
grant execute on function public.company_subscription_invoice_paid(uuid, text, text, text, bigint, boolean, timestamptz)
  to service_role;
revoke execute on function public.company_subscription_changed(uuid, text, text, text, timestamptz)
  from public, anon, authenticated;
grant execute on function public.company_subscription_changed(uuid, text, text, text, timestamptz)
  to service_role;
