begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('43000000-0000-0000-0000-0000000000a0', 'Created At', 'created-at', 'KR', 'ko', 'Asia/Seoul');

insert into public.task (id, company_id, title) values
  ('43000000-0000-0000-0000-0000000000f1', '43000000-0000-0000-0000-0000000000a0', 'a task nobody stamped');

select isnt(
  (select created_at from public.task where id = '43000000-0000-0000-0000-0000000000f1'),
  null,
  'a task records when it was created without anyone supplying the value'
);

update public.task
  set created_at = timestamptz '2024-01-02 03:04:05+09'
  where id = '43000000-0000-0000-0000-0000000000f1';

select is(
  (select created_at from public.task where id = '43000000-0000-0000-0000-0000000000f1'),
  timestamptz '2024-01-02 03:04:05+09',
  'history imported from the device keeps the day it actually happened'
);

update public.task
  set title = 'renamed'
  where id = '43000000-0000-0000-0000-0000000000f1';

select is(
  (select created_at from public.task where id = '43000000-0000-0000-0000-0000000000f1'),
  timestamptz '2024-01-02 03:04:05+09',
  'editing a task does not move the day it was created'
);

select * from finish();
rollback;
