begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email) values
  ('7a100000-0000-0000-0000-000000000011', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7a100000-0000-0000-0000-0000000000c1', 'Broadcast Company', 'broadcast-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('7a100000-0000-0000-0000-0000000000a1', '7a100000-0000-0000-0000-0000000000c1',
   'colleague@example.test', '7a100000-0000-0000-0000-000000000011', 'active');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"7a100000-0000-0000-0000-000000000011"}', true);
select set_config('realtime.topic', 'company:7a100000-0000-0000-0000-0000000000c1', true);

select throws_ok(
  $$insert into realtime.messages (topic, extension, event, payload, private)
    values ('company:7a100000-0000-0000-0000-0000000000c1', 'broadcast', 'task_written', '{}', true)$$,
  '42501',
  null,
  'a colleague cannot broadcast on the company channel in the record''s name'
);

select lives_ok(
  $$insert into realtime.messages (topic, extension, payload, private)
    values ('company:7a100000-0000-0000-0000-0000000000c1', 'presence', '{}', true)$$,
  'a colleague still shares their presence on the company channel'
);

select set_config('realtime.topic', 'company:7a100000-0000-0000-0000-0000000000c9', true);

select throws_ok(
  $$insert into realtime.messages (topic, extension, payload, private)
    values ('company:7a100000-0000-0000-0000-0000000000c9', 'presence', '{}', true)$$,
  '42501',
  null,
  'nor on another company''s channel'
);

select * from finish();
rollback;
