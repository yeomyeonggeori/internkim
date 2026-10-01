begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

select ok(
  not has_function_privilege('authenticated', 'public.claim_empty_box(uuid, text, text, timestamptz)', 'execute'),
  'a signed-in member cannot claim a box past the route that counts their guesses'
);

select ok(
  not has_function_privilege('anon', 'public.claim_empty_box(uuid, text, text, timestamptz)', 'execute'),
  'nor can an anonymous caller'
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
  (select outcome from public.claim_empty_box('7a300000-0000-0000-0000-0000000000c1',
     'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP', repeat('b', 64), now() - interval '10 minutes')),
  'wrong_code',
  'a code whose hash differs claims nothing'
);

select is(
  (select outcome from public.claim_empty_box('7a300000-0000-0000-0000-0000000000c1',
     'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP', repeat('a', 64), now() - interval '10 minutes')),
  'claimed',
  'the code the box was given claims it'
);

select is(
  (select count(*)::integer from public.empty_box where public_key = 'PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP'),
  0,
  'a claimed box leaves the list at once, so its code claims it only once'
);

select * from finish();
rollback;
