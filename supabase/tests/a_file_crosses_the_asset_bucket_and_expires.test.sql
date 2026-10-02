begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('44000000-0000-0000-0000-000000000001', 'transfer-a@example.test'),
  ('44000000-0000-0000-0000-000000000002', 'transfer-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('44000000-0000-0000-0000-0000000000a0', 'Transfer A', 'transfer-a', 'KR', 'ko', 'Asia/Seoul'),
  ('44000000-0000-0000-0000-0000000000b0', 'Transfer B', 'transfer-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('44000000-0000-0000-0000-0000000000a1', '44000000-0000-0000-0000-0000000000a0', 'transfer-a@example.test', '44000000-0000-0000-0000-000000000001', 'active'),
  ('44000000-0000-0000-0000-0000000000a2', '44000000-0000-0000-0000-0000000000a0', 'transfer-b@example.test', '44000000-0000-0000-0000-000000000002', 'active');

create function pg_temp.member_may_put(user_id text, object_name text)
returns boolean
language plpgsql
as $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', json_build_object('sub', user_id)::text, true);
  insert into storage.objects (bucket_id, name) values ('asset', object_name);
  reset role;
  return true;
exception when insufficient_privilege then
  reset role;
  return false;
end;
$$;

select ok(
  pg_temp.member_may_put('44000000-0000-0000-0000-000000000001', '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000a1/upload/1f0e'),
  'a member puts a file under their own upload prefix'
);

select ok(
  not pg_temp.member_may_put('44000000-0000-0000-0000-000000000001', '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000a2/upload/1f0e'),
  'a member cannot put a file under somebody else''s upload prefix'
);

select ok(
  not pg_temp.member_may_put('44000000-0000-0000-0000-000000000001', '44000000-0000-0000-0000-0000000000a0/shared/transfer/' || repeat('c', 64)),
  'a member cannot write a copy everyone reads, so nobody can plant the bytes behind a digest'
);

select ok(
  not pg_temp.member_may_put('44000000-0000-0000-0000-000000000001', '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000a1/transfer/' || repeat('d', 64)),
  'a member cannot write the copy the host makes of their own workspace file'
);

insert into storage.objects (bucket_id, name, created_at) values
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/transfer/old-media', now() - interval '8 days'),
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/transfer/new-media', now() - interval '1 day'),
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/transfer/staged-0c1d', now() - interval '8 days'),
  ('asset', '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000a1/transfer/old-file', now() - interval '9 days'),
  ('asset', '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000a1/upload/old-upload', now() - interval '10 days'),
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/attachment/refused-at-send', now() - interval '30 days'),
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/person-picture/face', now() - interval '30 days'),
  ('asset', '44000000-0000-0000-0000-0000000000b0/shared/transfer/other-company', now() - interval '30 days');

insert into public.agent (company_id, name, api_key_hash) values
  ('44000000-0000-0000-0000-0000000000a0', 'transfer host', 'transfer-host-hash');

create temporary table expired_as (caller text, name text);
grant insert on expired_as to authenticated;

create function pg_temp.expire_as(caller text, claims text, kept_days integer)
returns void
language plpgsql
as $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', claims, true);
  insert into expired_as select caller, name from public.expired_transfer_copies(kept_days) name;
  reset role;
end;
$$;

select pg_temp.expire_as('host', '{"sub":"host-a","app_metadata":{"company_id":"44000000-0000-0000-0000-0000000000a0","agent_key_hash":"transfer-host-hash"}}', 7);
select pg_temp.expire_as('member', '{"sub":"44000000-0000-0000-0000-000000000001"}', 1);

select is(
  array(select name from expired_as where caller = 'host' order by name),
  array[
    '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000a1/transfer/old-file',
    '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000a1/upload/old-upload',
    '44000000-0000-0000-0000-0000000000a0/shared/transfer/old-media',
    '44000000-0000-0000-0000-0000000000a0/shared/transfer/staged-0c1d'
  ],
  'the host is told only its own transfer copies older than it keeps them, never a refused attachment or a face'
);

select is(
  (select count(*) from expired_as where caller = 'member'),
  0::bigint,
  'a member is never told what to expire'
);

select * from finish();
rollback;
