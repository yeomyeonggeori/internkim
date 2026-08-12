begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

insert into auth.users (id, email) values
  ('43000000-0000-0000-0000-000000000001', 'asset-a@example.test'),
  ('43000000-0000-0000-0000-000000000002', 'asset-b@example.test'),
  ('43000000-0000-0000-0000-000000000003', 'asset-c@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('43000000-0000-0000-0000-0000000000a0', 'Asset A', 'asset-a', 'KR', 'ko', 'Asia/Seoul'),
  ('43000000-0000-0000-0000-0000000000b0', 'Asset B', 'asset-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.team (id, company_id, name) values
  ('43000000-0000-0000-0000-0000000000c1', '43000000-0000-0000-0000-0000000000a0', 'Makers'),
  ('43000000-0000-0000-0000-0000000000c2', '43000000-0000-0000-0000-0000000000a0', 'Sellers');

insert into public.circle (id, company_id, slug, name) values
  ('43000000-0000-0000-0000-0000000000f1', '43000000-0000-0000-0000-0000000000a0', 'incident', 'Incident'),
  ('43000000-0000-0000-0000-0000000000f2', '43000000-0000-0000-0000-0000000000a0', 'hiring', 'Hiring');

insert into public.member (id, company_id, email, user_id, status, team_id) values
  (
    '43000000-0000-0000-0000-0000000000a1',
    '43000000-0000-0000-0000-0000000000a0',
    'asset-a@example.test',
    '43000000-0000-0000-0000-000000000001',
    'active',
    '43000000-0000-0000-0000-0000000000c1'
  ),
  (
    '43000000-0000-0000-0000-0000000000a2',
    '43000000-0000-0000-0000-0000000000a0',
    'asset-c@example.test',
    '43000000-0000-0000-0000-000000000003',
    'active',
    '43000000-0000-0000-0000-0000000000c2'
  ),
  (
    '43000000-0000-0000-0000-0000000000b1',
    '43000000-0000-0000-0000-0000000000b0',
    'asset-b@example.test',
    '43000000-0000-0000-0000-000000000002',
    'active',
    null
  );

insert into public.circle_member (circle_id, member_id) values
  ('43000000-0000-0000-0000-0000000000f1', '43000000-0000-0000-0000-0000000000a1'),
  ('43000000-0000-0000-0000-0000000000f2', '43000000-0000-0000-0000-0000000000a2');

select is(
  public.asset_scope('43000000-0000-0000-0000-0000000000a0/person/43000000-0000-0000-0000-0000000000a1/note.txt'),
  'person',
  'the second segment names the scope'
);

select is(
  public.asset_scope('43000000-0000-0000-0000-0000000000a0/secrets/note.txt'),
  null,
  'a scope nobody defined is no scope, so the policy denies it'
);

select is(
  public.asset_company('../43000000-0000-0000-0000-0000000000a0/shared/x'),
  null,
  'a path that only contains a company does not belong to it'
);

insert into storage.objects (bucket_id, name, owner) values
  ('asset', '43000000-0000-0000-0000-0000000000a0/shared/picture-hash', null),
  ('asset', '43000000-0000-0000-0000-0000000000a0/person/43000000-0000-0000-0000-0000000000a1/mine.txt', null),
  ('asset', '43000000-0000-0000-0000-0000000000a0/person/43000000-0000-0000-0000-0000000000a2/theirs.txt', null),
  ('asset', '43000000-0000-0000-0000-0000000000a0/team/43000000-0000-0000-0000-0000000000c1/makers.txt', null),
  ('asset', '43000000-0000-0000-0000-0000000000a0/team/43000000-0000-0000-0000-0000000000c2/sellers.txt', null),
  ('asset', '43000000-0000-0000-0000-0000000000a0/circle/43000000-0000-0000-0000-0000000000f1/mine.txt', null),
  ('asset', '43000000-0000-0000-0000-0000000000a0/circle/43000000-0000-0000-0000-0000000000f2/theirs.txt', null),
  ('asset', '43000000-0000-0000-0000-0000000000a0/secrets/unnamed-scope.txt', null),
  ('asset', '43000000-0000-0000-0000-0000000000b0/shared/other-company.txt', null);

do $$
declare
  readable text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"43000000-0000-0000-0000-000000000001"}', true);

  select array_agg(name order by name) into readable from storage.objects where bucket_id = 'asset';

  assert readable = array[
    '43000000-0000-0000-0000-0000000000a0/circle/43000000-0000-0000-0000-0000000000f1/mine.txt',
    '43000000-0000-0000-0000-0000000000a0/person/43000000-0000-0000-0000-0000000000a1/mine.txt',
    '43000000-0000-0000-0000-0000000000a0/shared/picture-hash',
    '43000000-0000-0000-0000-0000000000a0/team/43000000-0000-0000-0000-0000000000c1/makers.txt'
  ], 'a member reads shared, their own person path, the circles they are in and their own team, and nothing else; saw ' || coalesce(array_to_string(readable, ', '), 'nothing');

  reset role;
end;
$$;

select pass('a member reads their own company shared assets');
select pass('a member reads their own person path');
select pass('a member reads their own team path');
select pass('a member reads a circle they are in, and not one they are not');
select pass('a member reads neither another person nor another team nor an unnamed scope');
select pass('a member reads nothing of another company');

do $$
declare
  planting_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"43000000-0000-0000-0000-000000000001"}', true);

  begin
    insert into storage.objects (bucket_id, name)
    values ('asset', '43000000-0000-0000-0000-0000000000a0/shared/planted');
  exception when insufficient_privilege then
    planting_blocked := true;
  end;
  assert planting_blocked, 'a member may read an asset but never write one';

  reset role;
end;
$$;

select pass('a member may not write an asset');

do $$
declare
  planting_blocked boolean := false;
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"43000000-0000-0000-0000-000000000001","app_metadata":{"company_id":"43000000-0000-0000-0000-0000000000a0"}}',
    true
  );

  insert into storage.objects (bucket_id, name)
  values ('asset', '43000000-0000-0000-0000-0000000000a0/person/43000000-0000-0000-0000-0000000000a2/written-by-host.txt');
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'the host writes any scope of the company it hosts';

  begin
    insert into storage.objects (bucket_id, name)
    values ('asset', '43000000-0000-0000-0000-0000000000b0/shared/planted');
  exception when insufficient_privilege then
    planting_blocked := true;
  end;
  assert planting_blocked, 'the host of one company does not write into another';

  reset role;
end;
$$;

select pass('the host writes the assets of the company it hosts, and no other');

select finish();
rollback;
