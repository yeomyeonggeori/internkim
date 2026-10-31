begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"000000dd-0000-0000-0000-000000000001"}', true);

select lives_ok(
  $$select public.push_device_claim('apns', 'abcdef0123456789', '{}'::jsonb)$$,
  'a phone claims its token in lower case'
);

select lives_ok(
  $$select public.push_device_claim('apns', 'ABCDEF0123456789', '{}'::jsonb)$$,
  'the same phone claims the same token in upper case'
);

reset role;

select is(
  (select count(*)::int from public.push_device where kind = 'apns' and lower(address) = 'abcdef0123456789'),
  1,
  'one phone is one device however its token is spelled'
);

select throws_ok(
  $$insert into public.push_device (member_id, kind, address, keys)
    values ('000000ee-0000-0000-0000-000000000001', 'apns-activity', 'FEDCBA9876543210', '{}')$$,
  '23514',
  null,
  'an Apple token written around the claim in upper case is refused'
);

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"000000dd-0000-0000-0000-000000000001"}', true);
select public.push_device_release('apns', 'ABCDEF0123456789');
reset role;

select is(
  (select count(*)::int from public.push_device where kind = 'apns' and address = 'abcdef0123456789'),
  0,
  'releasing the token in either case lets the phone go'
);

select * from finish();
rollback;
