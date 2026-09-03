-- The master profile a document prints from — legal name, representative, address,
-- bank account, the country-specific identifiers a letterhead carries — lived in
-- admind's own SQLite store, reachable only from the machine holding it. It is one
-- document with a slot per language rather than rows, so it is a column here and not
-- a table, checked the way work_hours is checked: a function the constraint calls.
--
-- The check is structural. It says what shape each slot has and refuses a key the
-- profile has no field for; it never reads a value to decide what it means, because
-- what an address or a 사업자등록번호 looks like is the country's business and not
-- this column's.
--
-- Grants and row level security are already what this needs: 20260803000016_api_grants
-- grants the table and every function in this schema, company_readable_by_member lets
-- a colleague read the row and company_updatable_by_admin lets an administrator write
-- it, and a new column on an existing table inherits all of them.

create function public.is_company_profile(profile jsonb)
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
        'foundedDate', 'capital', 'fiscalYearEnd', 'phone', 'fax', 'email',
        'website', 'updatedAt'
      ]) as plain(key)
      where profile ? plain.key
        and jsonb_typeof(profile -> plain.key) <> 'string'
    )
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

alter table public.company add column profile jsonb not null default '{}';

alter table public.company
  add constraint company_profile_is_a_master_profile
  check (public.is_company_profile(profile));

comment on column public.company.profile is
  'the master profile a company document prints from: a slot per language for the fields that have one, and the country-specific labels under legalAttributes';
