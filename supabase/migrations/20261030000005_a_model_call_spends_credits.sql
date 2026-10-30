-- A company without its own OpenRouter key reaches the model through the
-- model-gateway edge function, which calls with our key. These are the two
-- questions that function asks the record, each answered in one place.
--
-- company_member_credit_refusal says whether a member may start a call: the
-- company must have credits of either kind left, and the member must be under
-- the per-member ceiling, company.rules.creditPolicy.usageLimit or the default
-- the caller passes. It names the reason when the answer is no.
--
-- company_credit_usage_charged writes what a finished call cost. Subscription
-- credits are spent first, then permanent ones. A cost larger than both is
-- still the member's usage, and the part no balance covers is written as an
-- absorbed line first, so a balance never goes below zero.

create function public.company_member_credit_refusal(
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
  credits_left bigint;
  used bigint;
begin
  select coalesce((rules -> 'creditPolicy' ->> 'usageLimit')::bigint, default_usage_limit),
         subscription_credits + permanent_credits
  into usage_limit, credits_left
  from public.company
  where id = target_company;
  if not found then
    return 'no_company';
  end if;

  select credits_used into used
  from public.member
  where id = target_member and company_id = target_company;
  if not found then
    return 'not_a_member';
  end if;

  if credits_left <= 0 then
    return 'no_credits';
  end if;
  if used >= usage_limit then
    return 'usage_limit';
  end if;
  return null;
end;
$$;

create function public.company_credit_usage_charged(
  target_company uuid,
  target_member uuid,
  used_credits bigint
)
returns void
language plpgsql
security definer
set search_path = ''
as $$
declare
  subscription_left bigint;
  permanent_left bigint;
  from_subscription bigint;
  from_permanent bigint;
  absorbed bigint;
begin
  if used_credits <= 0 then
    return;
  end if;

  select subscription_credits, permanent_credits
  into subscription_left, permanent_left
  from public.company
  where id = target_company
  for update;
  if not found then
    raise exception 'no company % to charge', target_company
      using errcode = 'P0002';
  end if;

  from_subscription := least(used_credits, subscription_left);
  from_permanent := least(used_credits - from_subscription, permanent_left);
  absorbed := used_credits - from_subscription - from_permanent;

  if absorbed > 0 then
    insert into public.company_credit_entry (company_id, credit_kind, entry_kind, credits)
    values (target_company, 'subscription', 'absorbed', absorbed);
  end if;
  if from_subscription + absorbed > 0 then
    insert into public.company_credit_entry (company_id, member_id, credit_kind, entry_kind, credits)
    values (target_company, target_member, 'subscription', 'usage', -(from_subscription + absorbed));
  end if;
  if from_permanent > 0 then
    insert into public.company_credit_entry (company_id, member_id, credit_kind, entry_kind, credits)
    values (target_company, target_member, 'permanent', 'usage', -from_permanent);
  end if;
end;
$$;

revoke execute on function public.company_member_credit_refusal(uuid, uuid, bigint) from public, anon, authenticated;
revoke execute on function public.company_credit_usage_charged(uuid, uuid, bigint) from public, anon, authenticated;
grant execute on function public.company_member_credit_refusal(uuid, uuid, bigint) to service_role;
grant execute on function public.company_credit_usage_charged(uuid, uuid, bigint) to service_role;
