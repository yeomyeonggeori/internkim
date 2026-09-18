begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

select lives_ok(
  $$insert into public.push_device (member_id, kind, address, keys)
    values ('000000ee-0000-0000-0000-000000000001', 'apns-activity-start', 'start-token', '{}')$$,
  'the record keeps the token a phone starts its attendance Live Activity with'
);

select lives_ok(
  $$insert into public.push_device (member_id, kind, address, keys)
    values ('000000ee-0000-0000-0000-000000000001', 'apns-activity', 'activity-token', '{}')$$,
  'the record keeps the token a running Live Activity is ended with'
);

select throws_ok(
  $$insert into public.push_device (member_id, kind, address, keys)
    values ('000000ee-0000-0000-0000-000000000001', 'apns-activity-stale', 'other-token', '{}')$$,
  '23514',
  null,
  'a kind the record does not know is refused'
);

select * from finish();
rollback;
