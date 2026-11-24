begin;
create extension if not exists pgtap with schema extensions;
select plan(19);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7f000000-0000-0000-0000-0000000000e1', 'Company E', 'company-e', 'KR', 'ko', 'Asia/Seoul');

select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000e1', 'moving-from', repeat('f', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000e1', 'moving-to', repeat('t', 43), null, false);
select public.keep_claimed_box('7f000000-0000-0000-0000-0000000000e1', 'standing-by', repeat('s', 43), null, false);
update public.box set sealed_company_key = '{"version":1}'::jsonb where public_key in ('moving-to', 'standing-by');
update public.box set last_seen_at = now() - interval '5 seconds';
insert into storage.objects (bucket_id, name)
values ('box_backup', '7f000000-0000-0000-0000-0000000000e1/base/internkim-base-20261124T000000Z.json');

select is(
  public.make_box_serve('7f000000-0000-0000-0000-0000000000e1', 'moving-to'),
  'moving',
  'a box asked to serve while the service box is seen is moved to, not made to serve at once'
);
select is(
  (select string_agg(public_key || '=' || role, ',' order by public_key) from public.box
    where company_id = '7f000000-0000-0000-0000-0000000000e1'),
  'moving-from=service,moving-to=standby,standing-by=standby',
  'so the box that serves keeps serving, and keeps the store''s writes, until it has stopped'
);
select is(
  public.note_box_move('moving-to', 2, '0/3000148', 7),
  'waiting',
  'the box moved to cannot serve before the box moved from says where its logs end'
);

set local role authenticated;
select set_config('request.jwt.claims',
  '{"sub":"7f000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7f000000-0000-0000-0000-0000000000e1","fleet_id":"moving-from"}}',
  true);
select lives_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7f000000-0000-0000-0000-0000000000e1/wal/7300000000000000001/000000020000000003000000-0000000003000148')$$,
  'while it stops, the box moved from still keeps its last WAL in the store'
);
select throws_ok(
  $$select public.note_box_move('moving-from', 2, '0/3000148', 7)$$,
  '42501', null,
  'a box reports a move only through the server, which checks the box''s own signature'
);
reset role;

select is(public.note_box_move('moving-from', 2, '0/3000148', 7), 'stopped', 'the box moved from reports where it stopped');
select results_eq(
  $$select wal_timeline, wal_position, file_entry, stopped_at is not null from public.box_move
    where company_id = '7f000000-0000-0000-0000-0000000000e1'$$,
  $$values (2, '0/3000148'::pg_lsn, 7::bigint, true)$$,
  'and the record keeps it for the box moved to'
);
select is(
  public.note_box_move('moving-to', 2, '0/3000000', 7),
  'behind',
  'a box that replayed less than the box moved from kept does not serve'
);
select is(
  public.note_box_move('moving-to', 2, '0/3000148', 6),
  'behind',
  'nor one that replayed fewer file journal entries'
);
select is(
  (select role from public.box where public_key = 'moving-to'),
  'standby',
  'so it still stands by'
);
select is(public.note_box_move('moving-to', 2, '0/3000148', 7), 'serving', 'a box that replayed exactly as far as the other stopped serves');
select is(
  (select string_agg(public_key || '=' || role, ',' order by public_key) from public.box
    where company_id = '7f000000-0000-0000-0000-0000000000e1'),
  'moving-from=standby,moving-to=service,standing-by=standby',
  'and the box it moved from stands by'
);
select is(
  (select count(*)::integer from public.box_move where company_id = '7f000000-0000-0000-0000-0000000000e1'),
  0,
  'and the move is over'
);

set local role authenticated;
select set_config('request.jwt.claims',
  '{"sub":"7f000000-0000-0000-0000-00000000f0f0","app_metadata":{"company_id":"7f000000-0000-0000-0000-0000000000e1","fleet_id":"moving-from"}}',
  true);
select throws_ok(
  $$insert into storage.objects (bucket_id, name) values ('box_backup', '7f000000-0000-0000-0000-0000000000e1/wal/7300000000000000001/000000020000000003000148-0000000003000200')$$,
  '42501', null,
  'the box the company moved from has its writes to the store refused'
);
reset role;

select is(public.note_box_move('moving-from', 2, '0/3000200', 7), 'no_move', 'and a late report from it changes nothing');

update public.box set last_seen_at = now() - interval '5 seconds';
select is(public.make_box_serve('7f000000-0000-0000-0000-0000000000e1', 'standing-by'), 'moving', 'another move is asked');
select is(public.make_box_serve('7f000000-0000-0000-0000-0000000000e1', 'moving-to'), 'serving', 'asking for the box that serves calls it off');

insert into public.box_take_over (company_id, is_automatic, lapse_seconds)
values ('7f000000-0000-0000-0000-0000000000e1', false, 90);
select public.make_box_serve('7f000000-0000-0000-0000-0000000000e1', 'standing-by');
update public.box set last_seen_at = now() - interval '91 seconds' where public_key = 'moving-to';
select is(
  public.note_box_seen('standing-by', null),
  'service',
  'a move whose service box went silent ends as a take-over by the box it was asked to, once the lapse passed, even with take-over turned off'
);
update public.box set last_seen_at = now() - interval '91 seconds' where public_key = 'standing-by';
select is(
  public.make_box_serve('7f000000-0000-0000-0000-0000000000e1', 'moving-from'),
  'serving',
  'and a box asked to serve when the service box has lapsed takes over at once'
);

select * from finish();
rollback;
