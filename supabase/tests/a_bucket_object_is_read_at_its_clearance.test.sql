begin;
create extension if not exists pgtap with schema extensions;
select plan(15);

insert into auth.users (id, email) values
  ('53000000-0000-0000-0000-000000000001', 'bucket-member@example.test'),
  ('53000000-0000-0000-0000-000000000002', 'bucket-manager@example.test'),
  ('53000000-0000-0000-0000-000000000003', 'bucket-outsider@example.test'),
  ('53000000-0000-0000-0000-000000000004', 'bucket-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('53000000-0000-0000-0000-0000000000a0', 'Bucket A', 'bucket-a', 'KR', 'ko', 'Asia/Seoul'),
  ('53000000-0000-0000-0000-0000000000b0', 'Bucket B', 'bucket-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('53000000-0000-0000-0000-0000000000a1', '53000000-0000-0000-0000-0000000000a0', 'bucket-member@example.test', '53000000-0000-0000-0000-000000000001', 'active', false),
  ('53000000-0000-0000-0000-0000000000a2', '53000000-0000-0000-0000-0000000000a0', 'bucket-manager@example.test', '53000000-0000-0000-0000-000000000002', 'active', false),
  ('53000000-0000-0000-0000-0000000000a4', '53000000-0000-0000-0000-0000000000a0', 'bucket-admin@example.test', '53000000-0000-0000-0000-000000000004', 'active', true),
  ('53000000-0000-0000-0000-0000000000b1', '53000000-0000-0000-0000-0000000000b0', 'bucket-outsider@example.test', '53000000-0000-0000-0000-000000000003', 'active', true);

update public.member set clearance = 2 where id = '53000000-0000-0000-0000-0000000000a2';

insert into storage.objects (bucket_id, name, owner) values
  ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/0/' || repeat('a0', 32), null),
  ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/1/' || repeat('a1', 32), null),
  ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/2/' || repeat('a2', 32), null),
  ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/3/' || repeat('a3', 32) || '/text/01-summary.md', null),
  ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/9/' || repeat('a9', 32), null),
  ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/x/' || repeat('ab', 32), null),
  ('asset', '53000000-0000-0000-0000-0000000000b0/dataroom/1/' || repeat('b1', 32), null);

select is(
  public.asset_scope('53000000-0000-0000-0000-0000000000a0/dataroom/1/' || repeat('a1', 32)),
  'dataroom',
  'the second segment names the dataroom scope'
);

select is(
  public.asset_clearance('53000000-0000-0000-0000-0000000000a0/dataroom/2/' || repeat('a2', 32)),
  2::smallint,
  'the third segment is the clearance'
);

select is(
  public.asset_clearance('53000000-0000-0000-0000-0000000000a0/dataroom/9/' || repeat('a9', 32)),
  null,
  'a digit outside 0 to 3 is no clearance, so the policy denies it'
);

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000001"}', true);
  select array_agg(name order by name) into readable from storage.objects where bucket_id = 'asset';
  assert readable = array[
    '53000000-0000-0000-0000-0000000000a0/dataroom/0/' || repeat('a0', 32),
    '53000000-0000-0000-0000-0000000000a0/dataroom/1/' || repeat('a1', 32)
  ], 'a clearance 1 member reads 0 and 1; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('a clearance 1 member reads the objects at 0 and 1 and nothing above');

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000002"}', true);
  select array_agg(name order by name) into readable from storage.objects where bucket_id = 'asset';
  assert readable = array[
    '53000000-0000-0000-0000-0000000000a0/dataroom/0/' || repeat('a0', 32),
    '53000000-0000-0000-0000-0000000000a0/dataroom/1/' || repeat('a1', 32),
    '53000000-0000-0000-0000-0000000000a0/dataroom/2/' || repeat('a2', 32)
  ], 'a clearance 2 member reads up to 2; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('a clearance 2 member reads the objects at 0, 1 and 2');

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000004"}', true);
  select array_agg(name order by name) into readable from storage.objects where bucket_id = 'asset';
  assert readable = array[
    '53000000-0000-0000-0000-0000000000a0/dataroom/0/' || repeat('a0', 32),
    '53000000-0000-0000-0000-0000000000a0/dataroom/1/' || repeat('a1', 32),
    '53000000-0000-0000-0000-0000000000a0/dataroom/2/' || repeat('a2', 32),
    '53000000-0000-0000-0000-0000000000a0/dataroom/3/' || repeat('a3', 32) || '/text/01-summary.md'
  ], 'an administrator reads every clearance and no malformed path; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('an administrator reads the objects at every clearance, derived files included');
select pass('nobody reads an object whose clearance segment is not a digit from 0 to 3');

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000003"}', true);
  select array_agg(name order by name) into readable from storage.objects where bucket_id = 'asset';
  assert readable = array['53000000-0000-0000-0000-0000000000b0/dataroom/1/' || repeat('b1', 32)],
    'clearance opens nothing across companies; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');
  reset role;
end;
$$;

select pass('an administrator of another company reads nothing of this one');

do $$
declare
  rows_changed integer;
  planting_blocked boolean;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000001"}', true);

  insert into storage.objects (bucket_id, name)
  values ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/1/' || repeat('c1', 32));
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'a member adds an object at their own clearance';

  planting_blocked := false;
  begin
    insert into storage.objects (bucket_id, name)
    values ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/2/' || repeat('c2', 32));
  exception when insufficient_privilege then
    planting_blocked := true;
  end;
  assert planting_blocked, 'a member adds nothing above their clearance';

  planting_blocked := false;
  begin
    insert into storage.objects (bucket_id, name)
    values ('asset', '53000000-0000-0000-0000-0000000000b0/dataroom/1/' || repeat('c1', 32));
  exception when insufficient_privilege then
    planting_blocked := true;
  end;
  assert planting_blocked, 'a member adds nothing to another company';

  planting_blocked := false;
  begin
    insert into storage.objects (bucket_id, name)
    values ('asset', '53000000-0000-0000-0000-0000000000a0/dataroom/1/not-a-hash');
  exception when insufficient_privilege then
    planting_blocked := true;
  end;
  assert planting_blocked, 'a dataroom object is keyed by its hash';

  reset role;
end;
$$;

select pass('a member adds an object at their own clearance');
select pass('a member adds nothing above their clearance');
select pass('a member adds nothing to another company');
select pass('a dataroom object is keyed by its hash');

do $$
declare
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000002"}', true);

  update storage.objects set metadata = '{"replaced": true}'
  where bucket_id = 'asset'
    and name = '53000000-0000-0000-0000-0000000000a0/dataroom/2/' || repeat('a2', 32);
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'a member replaces an object at their own clearance';

  reset role;
end;
$$;

select pass('a member replaces an object at their own clearance');

select is(
  (select array_agg(cmd::text order by cmd::text) from pg_policies
   where schemaname = 'storage' and tablename = 'objects' and policyname like 'asset_dataroom%'),
  array['INSERT', 'UPDATE'],
  'a member adds and replaces in the dataroom and never deletes'
);

do $$
declare
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"53000000-0000-0000-0000-000000000001"}', true);

  update storage.objects set metadata = '{"replaced": true}'
  where bucket_id = 'asset'
    and name = '53000000-0000-0000-0000-0000000000a0/dataroom/2/' || repeat('a2', 32);
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member replaces nothing above their clearance';

  reset role;
end;
$$;

select pass('a member replaces nothing above their clearance');

select * from finish();
rollback;
