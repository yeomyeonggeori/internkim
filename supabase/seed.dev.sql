insert into auth.users (
  instance_id, id, aud, role, email, encrypted_password,
  email_confirmed_at, created_at, updated_at,
  raw_app_meta_data, raw_user_meta_data,
  confirmation_token, recovery_token, email_change,
  email_change_token_new, email_change_token_current,
  phone_change, phone_change_token, reauthentication_token
) values (
  '00000000-0000-0000-0000-000000000000',
  '000000dd-0000-0000-0000-000000000001',
  'authenticated', 'authenticated', 'pilot@example.test',
  crypt('pilot-password', gen_salt('bf')),
  now(), now(), now(),
  '{"provider":"email","providers":["email"]}', '{}',
  '', '', '', '', '', '', '', ''
) on conflict (id) do nothing;

insert into auth.identities (
  provider_id, user_id, identity_data, provider, last_sign_in_at, created_at, updated_at
) values (
  '000000dd-0000-0000-0000-000000000001',
  '000000dd-0000-0000-0000-000000000001',
  '{"sub":"000000dd-0000-0000-0000-000000000001","email":"pilot@example.test","email_verified":true}',
  'email', now(), now(), now()
) on conflict (provider, provider_id) do nothing;

insert into public.company (id, name, slug, country, locale, timezone, work_locations, leave_days)
values (
  '000000cc-0000-0000-0000-000000000001',
  'Pilot Company', 'pilot-company', 'KR', 'ko', 'Asia/Seoul',
  array['Headquarters', 'Branch'], 15
) on conflict (id) do nothing;

insert into public.member (id, company_id, email, user_id, status, is_admin)
values (
  '000000ee-0000-0000-0000-000000000001',
  '000000cc-0000-0000-0000-000000000001',
  'pilot@example.test',
  '000000dd-0000-0000-0000-000000000001',
  'active', true
) on conflict (id) do nothing;
