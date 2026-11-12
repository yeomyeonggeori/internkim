begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
  ('4a000000-0000-0000-0000-0000000000d1', 'participant-one@example.test'),
  ('4a000000-0000-0000-0000-0000000000d2', 'participant-outsider@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('4a000000-0000-0000-0000-0000000000a0', 'participant company', 'participant-company', 'KR', 'ko', 'Asia/Seoul'),
  ('4a000000-0000-0000-0000-0000000000b0', 'other company', 'participant-other-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, status, user_id) values
  ('4a000000-0000-0000-0000-0000000000a1', '4a000000-0000-0000-0000-0000000000a0', 'participant-one@example.test', 'active', '4a000000-0000-0000-0000-0000000000d1'),
  ('4a000000-0000-0000-0000-0000000000a2', '4a000000-0000-0000-0000-0000000000a0', 'participant-two@example.test', 'active', null),
  ('4a000000-0000-0000-0000-0000000000b1', '4a000000-0000-0000-0000-0000000000b0', 'participant-outsider@example.test', 'active', '4a000000-0000-0000-0000-0000000000d2');

insert into public.task (id, company_id, title, status) values
  ('4a000000-0000-0000-0000-000000000101', '4a000000-0000-0000-0000-0000000000a0', 'Shared work', 'planned'),
  ('4a000000-0000-0000-0000-000000000201', '4a000000-0000-0000-0000-0000000000b0', 'Other work', 'planned');

insert into public.task_participant (task_id, member_id) values
  ('4a000000-0000-0000-0000-000000000101', '4a000000-0000-0000-0000-0000000000a1'),
  ('4a000000-0000-0000-0000-000000000101', '4a000000-0000-0000-0000-0000000000a2'),
  ('4a000000-0000-0000-0000-000000000201', '4a000000-0000-0000-0000-0000000000b1');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"4a000000-0000-0000-0000-0000000000d1"}', true);

select is(
  (select count(*) from public.task_participant where task_id = '4a000000-0000-0000-0000-000000000101'),
  2::bigint,
  'a colleague reads every participant of a task in the company'
);
select is(
  (select count(*) from public.task_participant where task_id = '4a000000-0000-0000-0000-000000000201'),
  0::bigint,
  'a member reads no participant of another company''s task'
);

select set_config('request.jwt.claims', '{"sub":"4a000000-0000-0000-0000-0000000000d2"}', true);

select is(
  (select array_agg(member_id) from public.task_participant),
  array['4a000000-0000-0000-0000-0000000000b1'::uuid],
  'the outsider reads only their own company''s participants'
);

select set_config('request.jwt.claims', '{}', true);

select is(
  (select count(*) from public.task_participant),
  0::bigint,
  'someone who is no member reads no participant'
);

select * from finish();
rollback;
