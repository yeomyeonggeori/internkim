begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

select has_function('public', 'announce_task_write', 'announce: the task table has an announcer');
select has_trigger('public', 'task', 'task_write_announced', 'announce: task writes fire the announcer');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('77700000-0000-0000-0000-0000000000a0', 'Announce Company', 'announce-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title) values
  ('77700000-0000-0000-0000-0000000000b1', '77700000-0000-0000-0000-0000000000a0', 'Announced task');

select is(
  (select count(*)::integer from realtime.messages
    where topic = 'company:77700000-0000-0000-0000-0000000000a0' and event = 'task_written'),
  1,
  'announce: an insert lands one message on the company channel'
);

delete from public.task where id = '77700000-0000-0000-0000-0000000000b1';

select is(
  (select count(*)::integer from realtime.messages
    where topic = 'company:77700000-0000-0000-0000-0000000000a0' and event = 'task_written'),
  2,
  'announce: a delete lands another'
);

select * from finish();
rollback;
