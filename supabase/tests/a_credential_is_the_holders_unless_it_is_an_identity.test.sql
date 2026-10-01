begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000d1', 'holder@example.test'),
  ('00000000-0000-0000-0000-0000000000d2', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000d0', 'Company D', 'company-d', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000dd-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000d0', 'holder@example.test', '00000000-0000-0000-0000-0000000000d1', 'active', false),
  ('000000dd-0000-0000-0000-000000000002', '00000000-0000-0000-0000-0000000000d0', 'colleague@example.test', '00000000-0000-0000-0000-0000000000d2', 'active', false);

insert into public.credential (member_id, kind, external_id, name, settings) values
  ('000000dd-0000-0000-0000-000000000001', 'buzz-secret', 'public-key-hex', '', '{}'),
  ('000000dd-0000-0000-0000-000000000001', 'mail', '000000dd-0000-0000-0000-000000000001', '', '{}'),
  ('000000dd-0000-0000-0000-000000000001', 'api_key', 'ik_token_hash', '', '{"expiresAt": "2099-01-01T00:00:00Z"}'),
  ('000000dd-0000-0000-0000-000000000001', 'a-kind-nobody-named-yet', 'whatever', '', '{}');

select is(public.is_public_identity_credential('buzz-secret'), true,
  'who somebody is on the company messenger is public');
select is(public.is_public_identity_credential('buzz'), false,
  'the platform a company connects to is not a person''s identity');
select is(public.is_public_identity_credential('mattermost'), false,
  'a messenger nobody declares leaves its rows with the person they were issued to');
select is(public.is_public_identity_credential('mail'), false,
  'a mail account is not an identity a colleague may read');
select is(public.is_public_identity_credential('api_key'), false,
  'a personal key is not an identity a colleague may read');

create function pg_temp.kinds_a_colleague_reads() returns text language plpgsql as $$
declare
  seen text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000d2","role":"authenticated"}', true);
  select string_agg(kind, ',' order by kind) into seen
  from public.credential
  where member_id = '000000dd-0000-0000-0000-000000000001';
  reset role;
  return coalesce(seen, '');
end;
$$;

select is(pg_temp.kinds_a_colleague_reads(), 'buzz-secret',
  'a colleague reads the messenger identity and nothing else the holder was issued');

create function pg_temp.kinds_the_holder_reads() returns text language plpgsql as $$
declare
  seen text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000d1","role":"authenticated"}', true);
  select string_agg(kind, ',' order by kind) into seen
  from public.credential
  where member_id = '000000dd-0000-0000-0000-000000000001';
  reset role;
  return coalesce(seen, '');
end;
$$;

select is(pg_temp.kinds_the_holder_reads(), 'a-kind-nobody-named-yet,api_key,buzz-secret,mail',
  'the person a credential was issued to reads every one of them');

select isnt(pg_temp.kinds_a_colleague_reads(), pg_temp.kinds_the_holder_reads(),
  'the holder sees what a colleague does not');

select ok(pg_temp.kinds_a_colleague_reads() not like '%mail%',
  'a mail server, port and username stay with the person who signs in with them');

select ok(pg_temp.kinds_a_colleague_reads() not like '%a-kind-nobody-named-yet%',
  'a kind nobody has named is the holder''s until somebody says otherwise');

select throws_ok($block$do $$
begin
  set local role anon;
  perform public.is_public_identity_credential('buzz-secret');
  reset role;
end $$;$block$, '42501', null, 'the rule is not something the anonymous role may ask about');

select * from finish();
rollback;
