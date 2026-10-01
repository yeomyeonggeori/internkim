begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

select ok(
  not has_function_privilege('authenticated', 'public.claim_empty_box(uuid, text, text, timestamptz)', 'execute'),
  'a signed-in member cannot claim a box past the route that counts their guesses'
);

select ok(
  not has_function_privilege('anon', 'public.claim_empty_box(uuid, text, text, timestamptz)', 'execute'),
  'nor can an anonymous caller'
);

select ok(
  not has_function_privilege('authenticated', 'public.verify_box_pairing_code(uuid, text, text, text, timestamptz)', 'execute'),
  'a signed-in member cannot verify a code past the route that counts their guesses'
);

select ok(
  not has_table_privilege('authenticated', 'public.box_pairing_refusal', 'delete'),
  'a member cannot wipe the guesses their company made'
);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7a300000-0000-0000-0000-0000000000c1', 'Pairing Company', 'pairing-company', 'KR', 'ko', 'Asia/Seoul');

insert into public.empty_box (public_key, encryption_key, public_address, pairing_code_hash, pairing_code_expires_at) values
  ('PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP', 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM', '198.51.100.7',
   repeat('a', 64), now() + interval '15 minutes');

select is(
  (select outcome from public.verify_box_pairing_code('7a300000-0000-0000-0000-0000000000c1',
     'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP', repeat('b', 64), repeat('c', 64), now() - interval '10 minutes')),
  'wrong_code',
  'a code whose hash differs verifies nothing'
);

select is(
  (select outcome from public.verify_box_pairing_code('7a300000-0000-0000-0000-0000000000c1',
     'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP', repeat('a', 64), repeat('c', 64), now() - interval '10 minutes')),
  'verified',
  'the code the box was given verifies it'
);

select is(
  (select count(*)::integer from public.empty_box where public_key = 'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP'),
  1,
  'a verified code claims nothing until its owner confirms'
);

select is(
  (select count(*)::integer from public.box_pairing_refusal where company_id = '7a300000-0000-0000-0000-0000000000c1'),
  1,
  'a verified code is not counted as a wrong attempt'
);

select is(
  (select outcome from public.claim_empty_box('7a300000-0000-0000-0000-0000000000c1',
     'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP', repeat('a', 64), now() - interval '10 minutes')),
  'no_live_ticket',
  'the code itself no longer claims once it has been verified'
);

select is(
  (select outcome from public.claim_empty_box('7a300000-0000-0000-0000-0000000000c1',
     'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP', repeat('c', 64), now() - interval '10 minutes')),
  'claimed',
  'the ticket the verification issued claims it'
);

select is(
  (select count(*)::integer from public.empty_box where public_key = 'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP'),
  0,
  'a claimed box leaves the list at once, so its ticket claims it only once'
);

select * from finish();
rollback;
