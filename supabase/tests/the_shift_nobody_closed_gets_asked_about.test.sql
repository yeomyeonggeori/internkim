begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email) values
  ('53000000-0000-0000-0000-000000000001', 'forgot@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('53000000-0000-0000-0000-0000000000a0', 'Long Night', 'long-night', 'KR', 'ko', 'Asia/Seoul',
   '[{"name":"사무실"}]');

insert into public.member (id, company_id, user_id, email, name, status) values
  ('53000000-0000-0000-0000-0000000000b1', '53000000-0000-0000-0000-0000000000a0',
   '53000000-0000-0000-0000-000000000001', 'forgot@example.test', '이샘플', 'active');

insert into public.attendance (id, member_id, kind, location, occurred_at) values
  ('53000000-0000-0000-0000-0000000000c1', '53000000-0000-0000-0000-0000000000b1',
   'clock_in', '사무실', now() - interval '30 hours');

update public.attendance set asked_at = now()
  where id = '53000000-0000-0000-0000-0000000000c1';

select is(
  (select asked_at is not null from public.attendance
   where id = '53000000-0000-0000-0000-0000000000c1'),
  true,
  'the record remembers that somebody was asked about it'
);

select is(
  has_column_privilege('authenticated', 'public.attendance', 'asked_at', 'update'),
  false,
  'a person cannot decide they were already asked'
);

set local role authenticated;
set local request.jwt.claims = '{"sub":"53000000-0000-0000-0000-000000000001","role":"authenticated"}';

select is(
  (select asked_at is not null from public.attendance
   where id = '53000000-0000-0000-0000-0000000000c1'),
  true,
  'a person can still read their own record after being asked about it'
);

select * from finish();
rollback;
