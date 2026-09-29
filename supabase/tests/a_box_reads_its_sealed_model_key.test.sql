begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('4a000000-0000-0000-0000-0000000000a1', 'Keyed A', 'keyed-a', 'KR', 'ko', 'Asia/Seoul'),
  ('4a000000-0000-0000-0000-0000000000a2', 'Keyed B', 'keyed-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.credential (company_id, kind, external_id, settings) values
  ('4a000000-0000-0000-0000-0000000000a1', 'fleet', 'box-a', '{"sealedModelKey":{"ciphertext":"sealed-for-a"}}'),
  ('4a000000-0000-0000-0000-0000000000a2', 'fleet', 'box-b', '{"sealedModelKey":{"ciphertext":"sealed-for-b"}}');

set local role authenticated;

set local request.jwt.claims = '{"sub":"4a000000-0000-0000-0000-000000000001","role":"authenticated","app_metadata":{"company_id":"4a000000-0000-0000-0000-0000000000a1"}}';
select is(
  public.box_sealed_model_key() -> 'ciphertext',
  '"sealed-for-a"'::jsonb,
  'a host session of company A is given the key sealed for A'
);

set local request.jwt.claims = '{"sub":"4a000000-0000-0000-0000-000000000002","role":"authenticated","app_metadata":{"company_id":"4a000000-0000-0000-0000-0000000000a2"}}';
select is(
  public.box_sealed_model_key() -> 'ciphertext',
  '"sealed-for-b"'::jsonb,
  'a host session of company B is given the key sealed for B'
);
select isnt(
  public.box_sealed_model_key() -> 'ciphertext',
  '"sealed-for-a"'::jsonb,
  'and never the key sealed for A'
);

set local request.jwt.claims = '{"sub":"4a000000-0000-0000-0000-000000000003","role":"authenticated","app_metadata":{}}';
select is(
  public.box_sealed_model_key(),
  null::jsonb,
  'a token that names no company is given nothing'
);

set local role anon;
select throws_ok(
  $$select public.box_sealed_model_key()$$,
  '42501',
  null,
  'anon may not ask'
);

select * from finish();
rollback;
