begin;
create extension if not exists pgtap with schema extensions;
select plan(2);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7a200000-0000-0000-0000-0000000000c1', 'Expiry Company', 'expiry-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, status) values
  ('7a200000-0000-0000-0000-0000000000a1', '7a200000-0000-0000-0000-0000000000c1', 'holder@example.test', 'active');

select throws_ok(
  $$insert into public.credential (member_id, kind, name, external_id)
    values ('7a200000-0000-0000-0000-0000000000a1', 'api_key', 'forever', 'hash-forever')$$,
  '23514',
  null,
  'a personal access token cannot be kept without an expiry'
);

select lives_ok(
  $$insert into public.credential (member_id, kind, name, external_id, settings)
    values ('7a200000-0000-0000-0000-0000000000a1', 'api_key', 'ninety-days', 'hash-ninety',
            '{"expiresAt": "2027-01-01T00:00:00.000Z"}')$$,
  'one that says when it expires is kept'
);

select * from finish();
rollback;
