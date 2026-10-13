alter table public.company drop constraint company_profile_is_a_master_profile;

update public.company
set profile = profile - 'capital'
where jsonb_typeof(profile -> 'capital') = 'string' and btrim(profile ->> 'capital') = '';

do $$
declare
  unreadable text;
begin
  select string_agg(format('%s (%s)', id, profile ->> 'capital'), ', ') into unreadable
  from public.company
  where jsonb_typeof(profile -> 'capital') = 'string'
    and profile ->> 'capital' !~* '^\s*[$₩]?\s*[0-9][0-9,]*(\.[0-9]+)?\s*([A-Z]{3}|원|won)?\s*$';
  if unreadable is not null then
    raise exception 'capital written as prose cannot become an amount on its own: %', unreadable;
  end if;
end;
$$;

update public.company
set profile = jsonb_set(profile, '{capital}', jsonb_build_object(
  'amount', replace(substring(profile ->> 'capital' from '[0-9][0-9,]*(?:\.[0-9]+)?'), ',', '')::numeric,
  'currency', coalesce(
    substring(upper(profile ->> 'capital') from '[A-Z]{3}'),
    case when profile ->> 'capital' ~* '원|won|₩' then 'KRW' when profile ->> 'capital' ~ '\$' then 'USD' end,
    currency_code)))
where jsonb_typeof(profile -> 'capital') = 'string';

create or replace function public.is_money(money jsonb)
returns boolean
language sql
immutable
set search_path = ''
as $$
  select jsonb_typeof(money) = 'object'
    and (select array_agg(key order by key) from jsonb_object_keys(money) as field(key)) = array['amount', 'currency']
    and jsonb_typeof(money -> 'amount') = 'number'
    and money ->> 'currency' ~ '^[A-Z]{3}$';
$$;

create or replace function public.is_company_profile(profile jsonb)
returns boolean
language sql
immutable
set search_path = ''
as $$
  select jsonb_typeof(profile) = 'object'
    and not exists (
      select 1
      from jsonb_object_keys(profile) as field(key)
      where field.key not in (
        'name', 'brandName', 'slogan', 'description', 'representative',
        'representativeTitle', 'address', 'officeAddress', 'jurisdiction',
        'bankAccount', 'legalAttributes', 'foundedDate', 'capital',
        'fiscalYearEnd', 'employeeCount', 'phone', 'fax', 'email', 'website',
        'updatedAt'
      )
    )
    and not exists (
      select 1
      from unnest(array[
        'name', 'brandName', 'slogan', 'description', 'representative',
        'representativeTitle', 'address', 'officeAddress', 'jurisdiction',
        'bankAccount'
      ]) as localized(key)
      where profile ? localized.key
        and (
          jsonb_typeof(profile -> localized.key) <> 'object'
          or exists (
            select 1
            from jsonb_each(profile -> localized.key) as written(language, value)
            where jsonb_typeof(written.value) <> 'string'
          )
        )
    )
    and not exists (
      select 1
      from unnest(array[
        'foundedDate', 'fiscalYearEnd', 'phone', 'fax', 'email', 'website', 'updatedAt'
      ]) as plain(key)
      where profile ? plain.key
        and jsonb_typeof(profile -> plain.key) <> 'string'
    )
    and (not profile ? 'capital' or public.is_money(profile -> 'capital'))
    and (
      not profile ? 'employeeCount'
      or (
        jsonb_typeof(profile -> 'employeeCount') = 'number'
        and (profile ->> 'employeeCount')::numeric >= 0
        and (profile ->> 'employeeCount')::numeric
          = trunc((profile ->> 'employeeCount')::numeric)
      )
    )
    and (
      not profile ? 'legalAttributes'
      or (
        jsonb_typeof(profile -> 'legalAttributes') = 'object'
        and not exists (
          select 1
          from jsonb_each(profile -> 'legalAttributes') as slot(language, labels)
          where jsonb_typeof(slot.labels) <> 'object'
            or exists (
              select 1
              from jsonb_each(slot.labels) as written(label, value)
              where jsonb_typeof(written.value) <> 'string'
            )
        )
      )
    );
$$;

alter table public.company
  add constraint company_profile_is_a_master_profile
  check (public.is_company_profile(profile));
