begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7d000000-0000-0000-0000-0000000000e1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('7d000000-0000-0000-0000-0000000000e2', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.agent (company_id, name, api_key_hash, created_at, last_seen_at) values
  ('7d000000-0000-0000-0000-0000000000e1', 'company-computer', repeat('a', 64), now(), now()),
  ('7d000000-0000-0000-0000-0000000000e1', 'relay sample unit', repeat('b', 64), now() - interval '100 days', now() - interval '50 days'),
  ('7d000000-0000-0000-0000-0000000000e2', 'notify-config-sample', repeat('c', 64), now(), null);

select is(
  public.claim_box_with_connection_file(repeat('a', 64), 'company-computer', 'computer', repeat('k', 43)),
  '7d000000-0000-0000-0000-0000000000e1'::uuid,
  'the connection file claims the computer it is installed on, though the company holds another agent key'
);
select isnt(
  (select revoked_at from public.agent where api_key_hash = repeat('a', 64)),
  null,
  'and is spent by the claim'
);
select is(
  (select role from public.box where public_key = 'computer'),
  'service',
  'the computer serves the company it served on the file, since no box served it'
);
select is(
  (select sealing_key from public.box_keyring where company_id = '7d000000-0000-0000-0000-0000000000e1'),
  repeat('k', 43),
  'the company seals to the computer''s own key, as it does to a first box'
);
select is(
  (select sealed_company_key from public.box where public_key = 'computer'),
  null,
  'so the computer holds the company key without waiting for another box to hand it on'
);
select is(
  (select revoked_at from public.agent where api_key_hash = repeat('b', 64)),
  null,
  'the other agent key is not the claim''s to spend'
);
select is(
  public.company_served_by_connection_file('7d000000-0000-0000-0000-0000000000e1'),
  false,
  'and does not make the company look served by a connection file'
);

select is(
  public.company_served_by_connection_file('7d000000-0000-0000-0000-0000000000e2'),
  false,
  'an agent key a script holds is no connection file serving the company'
);
select is(
  public.keep_claimed_box('7d000000-0000-0000-0000-0000000000e2', 'first-box', repeat('m', 43), null, false),
  'service',
  'so the first box claimed by its code serves'
);
select is(
  public.make_box_serve('7d000000-0000-0000-0000-0000000000e2', 'first-box'),
  'serving',
  'and an administrator may make it serve'
);

select * from finish();
rollback;
