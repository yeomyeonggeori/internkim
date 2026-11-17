-- A company pays for its model calls one way at a time, and its subscription
-- says which: credits while the subscription is active or trialing, the
-- company's own OpenRouter key otherwise, when its computer holds one.
--
-- uses_own_model_key was a setting an administrator turned on and off, so a
-- company that held a key and then subscribed kept paying with the key until
-- someone found the switch. The database now sets it whenever
-- subscription_status changes, and when a company is created. Every reader
-- keeps the name it read before.
--
-- It stays an ordinary column so an operator can still set it by hand. A
-- hand-set value holds until the status next changes: the update trigger fires
-- only when subscription_status becomes something else, so a renewal or a
-- seat change that writes the same status again leaves it alone. Every write
-- of subscription_status goes through company_subscription_changed, which
-- updates the column the trigger watches.
--
-- A company's own session may not set it. The guard that keeps a company's
-- subscription and credits to the server now refuses an anon or authenticated
-- update that changes the payer too. It checks updates only: a company is
-- created by the server, and a payer sent with an insert is overwritten by the
-- insert trigger whoever sends it.
--
-- A subscription that is past due, unpaid, paused or ended does not pay, so
-- the key pays again from the computer's next start.
--
-- Existing companies are brought in line once. On production that changes
-- 0 of 4 companies.
--
-- To go back, drop the triggers company_payer_follows_its_subscription_on_insert
-- and company_payer_follows_its_subscription_on_change, then the function
-- public.company_payer_follows_its_subscription(), and restore the guard
-- internal.company_billing_is_written_by_the_server() as
-- 20261030000004_a_paid_period_grants_credits.sql defines it.

create function public.company_payer_follows_its_subscription()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  new.uses_own_model_key := new.subscription_status is null
    or new.subscription_status not in ('active', 'trialing');
  return new;
end;
$$;

revoke execute on function public.company_payer_follows_its_subscription() from public, anon, authenticated;

create trigger company_payer_follows_its_subscription_on_insert
  before insert on public.company
  for each row execute function public.company_payer_follows_its_subscription();

create trigger company_payer_follows_its_subscription_on_change
  before update of subscription_status on public.company
  for each row
  when (old.subscription_status is distinct from new.subscription_status)
  execute function public.company_payer_follows_its_subscription();

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
  if tg_op = 'UPDATE' and new.uses_own_model_key is distinct from old.uses_own_model_key then
    raise exception 'who pays for a company''s model calls is written by the server'
      using errcode = '42501';
  end if;
  return new;
end;
$$;

update public.company
set uses_own_model_key = (subscription_status is null or subscription_status not in ('active', 'trialing'))
where uses_own_model_key is distinct from
  (subscription_status is null or subscription_status not in ('active', 'trialing'));

comment on column public.company.uses_own_model_key is
  'whether the company''s own OpenRouter key pays for its model calls when its computer holds one: true unless its subscription is active or trialing, when credits pay through the model gateway. The subscription sets it whenever its status changes; an operator may set it by hand, and that holds until the status next changes. A company''s own session may not set it';
