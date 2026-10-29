-- A company that does not bring its own OpenRouter key pays a subscription and
-- spends credits through our key. One credit is a thousandth of a dollar of AI
-- cost. Stripe holds the money and the subscription; the record holds what the
-- credits did.
--
-- company_credit_entry is the ledger: one line per credit given or spent, kept
-- and never changed. company.subscription_credits and company.permanent_credits
-- are what is left of each kind, and member.credits_used is what a member has
-- spent this subscription period. They are written only by the line that moves
-- them, so a line and the balance it moved are one write. A member's usage goes
-- back to zero when the subscription period moves on.
--
-- Only the server writes any of it. authenticated may read its own company's
-- lines as an administrator, and a browser that tries to set a balance, a
-- usage or a Stripe identifier is refused.

alter table public.company
  add column stripe_customer_id text unique,
  add column stripe_subscription_id text unique,
  add column subscription_status text check (subscription_status in (
    'incomplete', 'incomplete_expired', 'trialing', 'active', 'past_due', 'canceled', 'unpaid', 'paused'
  )),
  add column subscription_ends_at timestamptz,
  add column subscription_credits bigint not null default 0 check (subscription_credits >= 0),
  add column permanent_credits bigint not null default 0 check (permanent_credits >= 0);

comment on column public.company.subscription_status is
  'the Stripe subscription status, as Stripe names it';
comment on column public.company.subscription_ends_at is
  'when the current subscription period ends; moving it forward starts a new period';
comment on column public.company.subscription_credits is
  'subscription credits left this period, the sum of the company''s subscription ledger lines';
comment on column public.company.permanent_credits is
  'purchased credits left, the sum of the company''s permanent ledger lines';

alter table public.member
  add column credits_used bigint not null default 0 check (credits_used >= 0);

comment on column public.member.credits_used is
  'credits this member spent in the current subscription period, both kinds together';

create table public.company_credit_entry (
  id uuid primary key default gen_random_uuid(),
  company_id uuid not null references public.company on delete cascade,
  member_id uuid,
  credit_kind text not null check (credit_kind in ('subscription', 'permanent')),
  entry_kind text not null check (entry_kind in ('grant', 'purchase', 'refund', 'usage', 'expiry', 'absorbed')),
  credits bigint not null check (credits <> 0),
  stripe_event_id text unique,
  created_at timestamptz not null default now(),
  constraint company_credit_entry_member_belongs_to_company
    foreign key (company_id, member_id)
    references public.member (company_id, id)
    on delete set null (member_id),
  constraint company_credit_entry_sign_follows_its_kind
    check (case entry_kind
      when 'usage' then credits < 0
      when 'refund' then credits < 0
      when 'expiry' then credits < 0
      else credits > 0
    end)
);

create index company_credit_entry_company_id_created_at_idx
  on public.company_credit_entry (company_id, created_at);

comment on table public.company_credit_entry is
  'one line of the company''s credit ledger: credits given (positive) or spent (negative); a line that would take a balance below zero is preceded by an absorbed line for the part we carry';

alter table public.company_credit_entry enable row level security;

create policy company_credit_entry_read_by_admin on public.company_credit_entry
  for select to authenticated
  using (company_id = internal.company_of_member(public.my_member()) and public.is_company_admin());

revoke all on public.company_credit_entry from anon, authenticated, service_role;
grant select on public.company_credit_entry to authenticated, service_role;
grant insert on public.company_credit_entry to service_role;

create function internal.company_credit_entry_applied()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  if new.credit_kind = 'subscription' then
    update public.company
    set subscription_credits = subscription_credits + new.credits
    where id = new.company_id;
  else
    update public.company
    set permanent_credits = permanent_credits + new.credits
    where id = new.company_id;
  end if;

  if new.entry_kind = 'usage' then
    update public.member
    set credits_used = credits_used - new.credits
    where id = new.member_id;
  end if;

  return new;
end;
$$;

create trigger company_credit_entry_applied
  after insert on public.company_credit_entry
  for each row execute function internal.company_credit_entry_applied();

create function internal.company_period_resets_credits_used()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  if old.subscription_ends_at is not null
    and new.subscription_ends_at > old.subscription_ends_at then
    update public.member
    set credits_used = 0
    where company_id = new.id;
  end if;
  return new;
end;
$$;

create trigger company_period_resets_credits_used
  after update of subscription_ends_at on public.company
  for each row execute function internal.company_period_resets_credits_used();

create function internal.company_billing_is_written_by_the_server()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  if current_user not in ('anon', 'authenticated') then
    return new;
  end if;
  if (new.stripe_customer_id, new.stripe_subscription_id, new.subscription_status,
      new.subscription_ends_at, new.subscription_credits, new.permanent_credits)
    is distinct from
     (case when tg_op = 'UPDATE' then
        (old.stripe_customer_id, old.stripe_subscription_id, old.subscription_status,
         old.subscription_ends_at, old.subscription_credits, old.permanent_credits)
      else (null::text, null::text, null::text, null::timestamptz, 0::bigint, 0::bigint) end) then
    raise exception 'a company''s subscription and credits are written by the server'
      using errcode = '42501';
  end if;
  return new;
end;
$$;

create trigger company_billing_is_written_by_the_server
  before insert or update on public.company
  for each row execute function internal.company_billing_is_written_by_the_server();

create function internal.member_credits_used_is_written_by_the_server()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  if current_user in ('anon', 'authenticated')
    and new.credits_used is distinct from (case when tg_op = 'UPDATE' then old.credits_used else 0 end) then
    raise exception 'a member''s credit usage is written by the server'
      using errcode = '42501';
  end if;
  return new;
end;
$$;

create trigger member_credits_used_is_written_by_the_server
  before insert or update on public.member
  for each row execute function internal.member_credits_used_is_written_by_the_server();

revoke execute on function internal.company_credit_entry_applied() from public, anon, authenticated;
revoke execute on function internal.company_period_resets_credits_used() from public, anon, authenticated;
revoke execute on function internal.company_billing_is_written_by_the_server() from public, anon, authenticated;
revoke execute on function internal.member_credits_used_is_written_by_the_server() from public, anon, authenticated;
