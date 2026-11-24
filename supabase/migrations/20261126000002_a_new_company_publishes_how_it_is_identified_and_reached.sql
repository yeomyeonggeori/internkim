create or replace function internal.data_room_fields_public_by_default()
returns table (kind text, field text) language sql immutable set search_path = ''
as $$
  values ('profile', 'name'), ('profile', 'brandName'), ('profile', 'slogan'), ('profile', 'description'),
    ('profile', 'website'), ('profile', 'foundedDate'), ('profile', 'activity'),
    ('profile', 'representative'), ('profile', 'legalAttributes'), ('profile', 'officeAddress'),
    ('profile', 'email'), ('profile', 'phone'), ('profile', 'fax'),
    ('record', 'product'), ('record', 'award'), ('record', 'history');
$$;

update public.company_field field set is_public = true
  where field.kind = 'profile'
    and field.field in ('representative', 'legalAttributes', 'officeAddress', 'email', 'phone', 'fax')
    and not field.is_public;
