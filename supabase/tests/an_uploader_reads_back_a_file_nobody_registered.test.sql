begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

insert into auth.users (id, email) values
  ('53000000-0000-0000-0000-000000000001', 'uploader@example.test'),
  ('53000000-0000-0000-0000-000000000002', 'colleague@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('53000000-0000-0000-0000-0000000000a0', 'Uploads', 'uploads', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('53000000-0000-0000-0000-0000000000a1', '53000000-0000-0000-0000-0000000000a0', 'uploader@example.test', '53000000-0000-0000-0000-000000000001', 'active', false),
  ('53000000-0000-0000-0000-0000000000a2', '53000000-0000-0000-0000-0000000000a0', 'colleague@example.test', '53000000-0000-0000-0000-000000000002', 'active', false);

select lives_ok($block$do $$
declare
  stored text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000001"}', true);
  insert into storage.objects (bucket_id, name, owner_id)
    values ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/PO/' || repeat('d', 64), '53000000-0000-0000-0000-000000000001')
    returning name into stored;
  reset role;
end $$;$block$, 'a member uploads into a category they read and the upload answers what it stored');

select throws_ok($block$do $$
declare
  stored text;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000001"}', true);
  insert into storage.objects (bucket_id, name, owner_id)
    values ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/FS/' || repeat('d', 64), '53000000-0000-0000-0000-000000000001')
    returning name into stored;
  reset role;
end $$;$block$, '42501', null, 'a member does not upload into a category they do not read');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000002"}', true);
select is_empty(
  $$ select 1 from storage.objects where name = '53000000-0000-0000-0000-0000000000a0/dataroom/PO/' || repeat('d', 64) $$,
  'a file nobody registered is read back by its uploader alone'
);
reset role;

select finish();
rollback;
