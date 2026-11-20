begin;
create extension if not exists pgtap with schema extensions;
select plan(18);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7f000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('7f000000-0000-0000-0000-0000000000c2', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul'),
  ('7f000000-0000-0000-0000-0000000000c3', 'Company C', 'company-c', 'KR', 'ko', 'Asia/Seoul');

insert into public.agent (company_id, name, api_key_hash) values
  ('7f000000-0000-0000-0000-0000000000c1', 'company-computer', repeat('a', 64)),
  ('7f000000-0000-0000-0000-0000000000c2', 'company-computer', repeat('b', 64)),
  ('7f000000-0000-0000-0000-0000000000c3', 'company-computer', repeat('c', 64)),
  ('7f000000-0000-0000-0000-0000000000c3', 'another-computer', repeat('d', 64));

insert into public.empty_box (public_key, encryption_key, public_address)
values (repeat('p', 43), repeat('e', 43), '198.51.100.7');

select throws_ok(
  $$ select * from public.claim_box_with_connection_file(repeat('a', 64), 'company-computer', repeat('p', 43), 'not an encryption key', false) $$,
  '23514',
  null,
  'a claim the record cannot keep fails'
);
select is(
  (select revoked_at from public.agent where api_key_hash = repeat('a', 64)),
  null,
  'and leaves the connection file standing, so the computer that holds it still signs in'
);

select is(
  (select outcome from public.claim_box_with_connection_file(repeat('d', 64), 'company-computer', repeat('p', 43), repeat('e', 43), false)),
  'not_standing',
  'a key of another name claims nothing'
);

select is(
  (select row(outcome, claimed_company)::text from public.claim_box_with_connection_file(repeat('a', 64), 'company-computer', repeat('p', 43), repeat('e', 43), false)),
  row('claimed', '7f000000-0000-0000-0000-0000000000c1'::uuid)::text,
  'the connection file of a company with no other computer claims the box without replacing anything'
);
select isnt(
  (select revoked_at from public.agent where api_key_hash = repeat('a', 64)),
  null,
  'and is spent'
);
select is(
  (select role from public.box where public_key = repeat('p', 43)),
  'service',
  'the box serves the company'
);
select is(
  (select count(*)::integer from public.empty_box where public_key = repeat('p', 43)),
  0,
  'and is off the list of empty boxes'
);
select is(
  (select outcome from public.claim_box_with_connection_file(repeat('a', 64), 'company-computer', repeat('q', 43), repeat('f', 43), false)),
  'not_standing',
  'a spent file claims nothing more'
);

select is(
  public.keep_claimed_box('7f000000-0000-0000-0000-0000000000c2', 'kim-mini-b', repeat('k', 43), 'kimmini', false),
  'standby',
  'a box added beside the computer company B''s connection file serves stands by'
);
select is(
  (select outcome from public.claim_box_with_connection_file(repeat('b', 64), 'company-computer', repeat('r', 43), repeat('g', 43), false)),
  'company_has_other_computers',
  'a claim that does not replace is refused while the company has another box'
);
select is(
  (select revoked_at from public.agent where api_key_hash = repeat('b', 64)),
  null,
  'and spends nothing'
);
select is(
  (select array_agg(public_key || ':' || role) from public.box where company_id = '7f000000-0000-0000-0000-0000000000c2'),
  array['kim-mini-b:standby'],
  'the other box stays as it was'
);
select is(
  (select count(*)::integer from public.box where public_key = repeat('r', 43)),
  0,
  'and the refused box is kept by no company'
);

select is(
  (select outcome from public.claim_box_with_connection_file(repeat('c', 64), 'company-computer', repeat('s', 43), repeat('h', 43), false)),
  'company_has_other_computers',
  'a claim that does not replace is refused while another key of the company stands'
);
select is(
  (select count(*)::integer from public.agent where company_id = '7f000000-0000-0000-0000-0000000000c3' and revoked_at is null),
  2,
  'and both keys still stand'
);

select is(
  (select outcome from public.claim_box_with_connection_file(repeat('b', 64), 'company-computer', repeat('r', 43), repeat('g', 43), true)),
  'claimed',
  'a replacing claim, as an install asks, takes company B'
);
select is(
  (select array_agg(public_key || ':' || role) from public.box where company_id = '7f000000-0000-0000-0000-0000000000c2'),
  array[repeat('r', 43) || ':service'],
  'and lets the other box go'
);
select isnt(
  (select revoked_at from public.agent where api_key_hash = repeat('b', 64)),
  null,
  'and spends the key'
);

select * from finish();
rollback;
