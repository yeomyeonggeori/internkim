begin;
create extension if not exists pgtap with schema extensions;
select plan(1);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000e1', 'holder@example.test'),
  ('00000000-0000-0000-0000-0000000000e2', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000e0', 'Company E', 'company-e', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000ee-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000e0', 'holder@example.test', '00000000-0000-0000-0000-0000000000e1', 'active', false),
  ('000000ee-0000-0000-0000-000000000002', '00000000-0000-0000-0000-0000000000e0', 'colleague@example.test', '00000000-0000-0000-0000-0000000000e2', 'active', false);

insert into public.credential (member_id, kind, external_id, name) values
  ('000000ee-0000-0000-0000-000000000001', 'buzz-secret', 'public-key-hex', ''),
  ('000000ee-0000-0000-0000-000000000001', 'api_key', 'ik_token_hash', '');

select lives_ok($block$do $$
declare
  seen integer;
begin
  set local role anon;
  perform set_config('request.jwt.claims', '{}', true);
  select count(*) into seen from public.credential;
  assert seen = 0, 'an anonymous read answers zero rows';
  reset role;
end $$;$block$, 'an anonymous read of credential raises nothing');

select * from finish();
rollback;
