begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('99700000-0000-0000-0000-0000000000a0', 'Board Move Company', 'board-move-company', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, name, email, status) values
  ('99700000-0000-0000-0000-0000000000c1', '99700000-0000-0000-0000-0000000000a0', '이샘플', 'board-move-sample@example.com', 'active'),
  ('99700000-0000-0000-0000-0000000000c2', '99700000-0000-0000-0000-0000000000a0', '박예시', 'board-move-example@example.com', 'active');

create temporary table today_in_seoul as select (now() at time zone 'Asia/Seoul')::date as day;

create function pg_temp.day_at(offset_days integer) returns timestamptz language sql as $$
  select ((select day from today_in_seoul) + offset_days)::timestamp at time zone 'Asia/Seoul';
$$;

insert into public.task (id, company_id, title, status, starts_at, ends_at, is_whole_day, notify_minutes_before) values
  ('99700000-0000-0000-0000-0000000000b1', '99700000-0000-0000-0000-0000000000a0', 'Starts next week', 'planned', pg_temp.day_at(7), pg_temp.day_at(9), false, null),
  ('99700000-0000-0000-0000-0000000000b2', '99700000-0000-0000-0000-0000000000a0', 'Finished last week', 'completed', pg_temp.day_at(-9), pg_temp.day_at(-8), false, null),
  ('99700000-0000-0000-0000-0000000000b3', '99700000-0000-0000-0000-0000000000a0', 'Running across today', 'in_progress', pg_temp.day_at(-2), pg_temp.day_at(3), false, 30),
  ('99700000-0000-0000-0000-0000000000b4', '99700000-0000-0000-0000-0000000000a0', 'Whole days across today', 'in_progress', pg_temp.day_at(-1), pg_temp.day_at(4), true, null),
  ('99700000-0000-0000-0000-0000000000b5', '99700000-0000-0000-0000-0000000000a0', 'Paused long ago', 'paused', pg_temp.day_at(-20), pg_temp.day_at(-15), false, null),
  ('99700000-0000-0000-0000-0000000000b6', '99700000-0000-0000-0000-0000000000a0', 'Nobody asked', 'planned', null, null, false, null);
insert into public.task (id, company_id, title, status, starts_at, ends_at, requester_id) values
  ('99700000-0000-0000-0000-0000000000b7', '99700000-0000-0000-0000-0000000000a0', 'Asked by another', 'planned', null, null, null),
  ('99700000-0000-0000-0000-0000000000b8', '99700000-0000-0000-0000-0000000000a0', 'Asked of oneself', 'requested', null, null, '99700000-0000-0000-0000-0000000000c1');
update public.task set status = 'requested', requester_id = '99700000-0000-0000-0000-0000000000c2'
  where id = '99700000-0000-0000-0000-0000000000b7';
insert into public.task_participant (task_id, member_id) values
  ('99700000-0000-0000-0000-0000000000b7', '99700000-0000-0000-0000-0000000000c1'),
  ('99700000-0000-0000-0000-0000000000b8', '99700000-0000-0000-0000-0000000000c1');

update public.task set status = 'in_progress' where id = '99700000-0000-0000-0000-0000000000b1';
select is(
  (select (status::text, public.task_first_day(task), public.task_last_day(task)) from public.task where id = '99700000-0000-0000-0000-0000000000b1'),
  ('in_progress'::text, (select day from today_in_seoul), (select day + 9 from today_in_seoul)),
  'named status: starting a task early moves its start to now and keeps its end');

update public.task set status = 'in_progress' where id = '99700000-0000-0000-0000-0000000000b2';
select is(
  (select (status::text, (starts_at = pg_temp.day_at(-9)), ends_at) from public.task where id = '99700000-0000-0000-0000-0000000000b2'),
  ('in_progress'::text, true, null::timestamptz),
  'named status: reopening a finished task clears the end it passed and keeps its start');

update public.task set status = 'planned' where id = '99700000-0000-0000-0000-0000000000b3';
select is(
  (select (status::text, starts_at, ends_at = pg_temp.day_at(3), notify_minutes_before) from public.task where id = '99700000-0000-0000-0000-0000000000b3'),
  ('planned'::text, null::timestamptz, true, null::integer),
  'named status: planning a started task clears its start and the reminder that needs it, and keeps the end ahead');

update public.task set status = 'planned' where id = '99700000-0000-0000-0000-0000000000b4';
select is(
  (select (status::text, starts_at, is_whole_day, public.task_last_day(task)) from public.task where id = '99700000-0000-0000-0000-0000000000b4'),
  ('planned'::text, null::timestamptz, false, (select day + 3 from today_in_seoul)),
  'named status: planning a whole-day task keeps its last day once the start is cleared');

update public.task set status = 'completed' where id = '99700000-0000-0000-0000-0000000000b4';
select is(
  (select (status::text, public.task_last_day(task), starts_at = ends_at) from public.task where id = '99700000-0000-0000-0000-0000000000b4'),
  ('completed'::text, (select day from today_in_seoul), true),
  'named status: completing a task due later ends it today');

select throws_ok(
  $$update public.task set status = 'requested' where id = '99700000-0000-0000-0000-0000000000b6'$$,
  '23514', 'a task nobody asked for cannot be requested',
  'request: a task nobody asked for cannot be moved into requested');

select throws_ok(
  $$update public.task set status = 'requested' where id = '99700000-0000-0000-0000-0000000000b5'$$,
  '23514', 'a task nobody asked for cannot be requested',
  'request: a paused task nobody asked for cannot be moved into requested either');

select is(
  (select (status::text, starts_at = pg_temp.day_at(-20), ends_at = pg_temp.day_at(-15)) from public.task where id = '99700000-0000-0000-0000-0000000000b5'),
  ('paused'::text, true, true),
  'named status: a refused request leaves the task as it was');

update public.task set status = 'paused' where id = '99700000-0000-0000-0000-0000000000b7';
update public.task set status = 'requested' where id = '99700000-0000-0000-0000-0000000000b7';
select is(
  (select (status::text, requester_id) from public.task where id = '99700000-0000-0000-0000-0000000000b7'),
  ('requested'::text, '99700000-0000-0000-0000-0000000000c2'::uuid),
  'request: work someone else asked for moves back into requested');

select is(
  (select (status::text, requester_id) from public.task where id = '99700000-0000-0000-0000-0000000000b8'),
  ('planned'::text, null::uuid),
  'request: asking only oneself is no request, so it becomes planned with nobody asking');

insert into public.task_participant (task_id, member_id) values
  ('99700000-0000-0000-0000-0000000000b7', '99700000-0000-0000-0000-0000000000c2');
delete from public.task_participant
  where task_id = '99700000-0000-0000-0000-0000000000b7' and member_id = '99700000-0000-0000-0000-0000000000c1';
select is(
  (select (status::text, requester_id) from public.task where id = '99700000-0000-0000-0000-0000000000b7'),
  ('planned'::text, null::uuid),
  'request: a request left with only the one who asked stops being one');

create temporary table every_move as
select shape.name as shape, from_status, to_status, shape.starts_at, shape.ends_at, shape.is_whole_day
from (values
  ('none', null::timestamptz, null::timestamptz, false),
  ('past', pg_temp.day_at(-9), pg_temp.day_at(-8), false),
  ('across today', pg_temp.day_at(-1), pg_temp.day_at(2), false),
  ('ends today', pg_temp.day_at(0), pg_temp.day_at(0) + interval '18 hours', false),
  ('future', pg_temp.day_at(8), pg_temp.day_at(9), false),
  ('whole days ahead', pg_temp.day_at(8), pg_temp.day_at(10), true),
  ('whole days past', pg_temp.day_at(-6), pg_temp.day_at(-4), true),
  ('end only, ahead', null, pg_temp.day_at(5), false)
) as shape(name, starts_at, ends_at, is_whole_day)
cross join unnest(array['planned', 'in_progress', 'completed', 'paused', 'requested']) as from_status
cross join unnest(array['planned', 'in_progress', 'completed', 'paused', 'requested']) as to_status
where from_status <> to_status;

create temporary table move_failures (shape text, from_status text, to_status text, failure text);

do $$
declare
  move record;
  moved uuid;
begin
  for move in select * from every_move loop
    insert into public.task (company_id, title, status, starts_at, ends_at, is_whole_day, requester_id)
    values ('99700000-0000-0000-0000-0000000000a0', 'every move', move.from_status::public.task_status,
      move.starts_at, move.ends_at, move.is_whole_day,
      case when move.from_status = 'requested' then '99700000-0000-0000-0000-0000000000c2'::uuid end)
    returning id into moved;
    begin
      update public.task set status = move.to_status::public.task_status where id = moved;
    exception when others then
      if move.to_status = 'requested' and sqlerrm = 'a task nobody asked for cannot be requested' then
        delete from public.task where id = moved;
      else
        insert into move_failures values (move.shape, move.from_status, move.to_status, sqlerrm);
      end if;
    end;
  end loop;
end;
$$;

select is(
  (select count(*)::integer from move_failures),
  0,
  'every move: no column refuses any other column, whatever the dates, except a request nobody made');

select is(
  (select count(*)::integer
    from public.task
    where title = 'every move'
      and status::text in ('planned', 'in_progress', 'completed')
      and not case status::text
        when 'in_progress' then (starts_at is null or public.task_first_day(task) <= (select day from today_in_seoul))
          and (ends_at is null or public.task_last_day(task) >= (select day from today_in_seoul))
        when 'completed' then ends_at is not null and public.task_last_day(task) <= (select day from today_in_seoul)
        else ends_at is null or public.task_last_day(task) >= (select day from today_in_seoul)
      end),
  0,
  'every move: each task lands with dates its column is true of');

select * from finish();
rollback;
