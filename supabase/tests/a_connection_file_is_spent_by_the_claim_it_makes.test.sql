begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7d000000-0000-0000-0000-0000000000c3', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('7d000000-0000-0000-0000-0000000000c4', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.agent (company_id, name, api_key_hash) values
  ('7d000000-0000-0000-0000-0000000000c3', 'company-computer', repeat('a', 64)),
  ('7d000000-0000-0000-0000-0000000000c4', 'company-computer', repeat('b', 64)),
  ('7d000000-0000-0000-0000-0000000000c4', 'another-caller', repeat('c', 64));

select public.keep_claimed_box('7d000000-0000-0000-0000-0000000000c3', 'serving', repeat('s', 43), 'kimmini', false);
update public.box_keyring
  set sealed_model_key = '{"version":1,"recipient":"sssssssssssssssssssssssssssssssssssssssssss","enc":"e","ciphertext":"c"}'::jsonb
  where company_id = '7d000000-0000-0000-0000-0000000000c3';

select is(
  public.claim_box_with_connection_file(repeat('a', 64), 'company-computer', 'computer', repeat('k', 43)),
  '7d000000-0000-0000-0000-0000000000c3'::uuid,
  'the connection file claims the computer it is installed on into its company'
);
select isnt(
  (select revoked_at from public.agent where api_key_hash = repeat('a', 64)),
  null,
  'and is spent by the claim'
);
select is(
  (select role from public.box where public_key = 'computer'),
  'standby',
  'the computer stands by beside the box that serves, as one added by its code would'
);
select is(
  (select role from public.box where public_key = 'serving'),
  'service',
  'and the box that served keeps serving'
);
select is(
  (select sealing_key from public.box_keyring where company_id = '7d000000-0000-0000-0000-0000000000c3'),
  repeat('s', 43),
  'the company keeps sealing to the key that box holds'
);
select isnt(
  (select sealed_model_key from public.box_keyring where company_id = '7d000000-0000-0000-0000-0000000000c3'),
  null,
  'so the model key sealed to it is kept'
);
select is(
  (select sealed_company_key from public.box where public_key = 'computer'),
  null,
  'and the computer waits for that box to hand it the company key'
);

select is(
  public.claim_box_with_connection_file(repeat('a', 64), 'company-computer', 'another-computer', repeat('m', 43)),
  null,
  'a spent connection file claims nothing more'
);

select is(
  public.claim_box_with_connection_file(repeat('c', 64), 'company-computer', 'stranger', repeat('t', 43)),
  null,
  'a key the company holds under another name claims nothing'
);

select throws_ok(
  $$select public.claim_box_with_connection_file(repeat('b', 64), 'company-computer', 'refused', 'not a key')$$,
  '23514',
  null,
  'a box the record cannot keep is not claimed'
);
select is(
  (select revoked_at from public.agent where api_key_hash = repeat('b', 64)),
  null,
  'and its connection file still stands'
);

select * from finish();
rollback;
