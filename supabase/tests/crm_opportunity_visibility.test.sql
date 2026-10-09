begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

insert into auth.users (id, email) values
  ('5a200000-0000-0000-0000-0000000000d1', 'admin@example.test'),
  ('5a200000-0000-0000-0000-0000000000d2', 'seller@example.test'),
  ('5a200000-0000-0000-0000-0000000000d3', 'teammate@example.test'),
  ('5a200000-0000-0000-0000-0000000000d4', 'head@example.test'),
  ('5a200000-0000-0000-0000-0000000000d5', 'engineer@example.test'),
  ('5a200000-0000-0000-0000-0000000000d6', 'loner@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('5a200000-0000-0000-0000-0000000000a0', 'Deal Visibility', 'deal-visibility', 'KR', 'ko', 'Asia/Seoul');

insert into public.team (id, company_id, name) values
  ('5a200000-0000-0000-0000-0000000000e1', '5a200000-0000-0000-0000-0000000000a0', '사업본부'),
  ('5a200000-0000-0000-0000-0000000000e2', '5a200000-0000-0000-0000-0000000000a0', '영업팀'),
  ('5a200000-0000-0000-0000-0000000000e3', '5a200000-0000-0000-0000-0000000000a0', '개발팀');

update public.team set parent_team_id = '5a200000-0000-0000-0000-0000000000e1'
where id = '5a200000-0000-0000-0000-0000000000e2';

insert into public.member (id, company_id, email, user_id, status, is_admin, team_id) values
  ('5a200000-0000-0000-0000-000000000001', '5a200000-0000-0000-0000-0000000000a0', 'admin@example.test', '5a200000-0000-0000-0000-0000000000d1', 'active', true, null),
  ('5a200000-0000-0000-0000-000000000002', '5a200000-0000-0000-0000-0000000000a0', 'seller@example.test', '5a200000-0000-0000-0000-0000000000d2', 'active', false, '5a200000-0000-0000-0000-0000000000e2'),
  ('5a200000-0000-0000-0000-000000000003', '5a200000-0000-0000-0000-0000000000a0', 'teammate@example.test', '5a200000-0000-0000-0000-0000000000d3', 'active', false, '5a200000-0000-0000-0000-0000000000e2'),
  ('5a200000-0000-0000-0000-000000000004', '5a200000-0000-0000-0000-0000000000a0', 'head@example.test', '5a200000-0000-0000-0000-0000000000d4', 'active', false, '5a200000-0000-0000-0000-0000000000e1'),
  ('5a200000-0000-0000-0000-000000000005', '5a200000-0000-0000-0000-0000000000a0', 'engineer@example.test', '5a200000-0000-0000-0000-0000000000d5', 'active', false, '5a200000-0000-0000-0000-0000000000e3'),
  ('5a200000-0000-0000-0000-000000000006', '5a200000-0000-0000-0000-0000000000a0', 'loner@example.test', '5a200000-0000-0000-0000-0000000000d6', 'active', false, null);

insert into public.organization (id, company_id, name) values
  ('5a200000-0000-0000-0000-0000000000b1', '5a200000-0000-0000-0000-0000000000a0', '견본상사'),
  ('5a200000-0000-0000-0000-0000000000b2', '5a200000-0000-0000-0000-0000000000a0', '예시물산');

insert into public.contact (id, company_id, organization_id, name, email, phone) values
  ('5a200000-0000-0000-0000-0000000000c1', '5a200000-0000-0000-0000-0000000000a0', '5a200000-0000-0000-0000-0000000000b1', '이샘플', 'sample@example.com', '010-0000-0000');

insert into public.opportunity (id, company_id, organization_id, contact_id, name, pipeline_id, stage_id, owner_id, created_by, amount_minor, currency_code) values
  ('5a200000-0000-0000-0000-000000000101', '5a200000-0000-0000-0000-0000000000a0', '5a200000-0000-0000-0000-0000000000b1', '5a200000-0000-0000-0000-0000000000c1', '영업팀 딜', 'sales', 'waiting', '5a200000-0000-0000-0000-000000000002', '5a200000-0000-0000-0000-000000000001', 50000000, 'KRW'),
  ('5a200000-0000-0000-0000-000000000102', '5a200000-0000-0000-0000-0000000000a0', '5a200000-0000-0000-0000-0000000000b1', null, '담당자 없는 딜', 'sales', 'waiting', null, '5a200000-0000-0000-0000-000000000005', 7000000, 'KRW');

insert into public.task (id, company_id, title, organization_id, opportunity_id) values
  ('5a200000-0000-0000-0000-000000000201', '5a200000-0000-0000-0000-0000000000a0', '영업팀 딜 미팅', '5a200000-0000-0000-0000-0000000000b1', '5a200000-0000-0000-0000-000000000101'),
  ('5a200000-0000-0000-0000-000000000202', '5a200000-0000-0000-0000-0000000000a0', '영업팀 딜 시연', '5a200000-0000-0000-0000-0000000000b1', '5a200000-0000-0000-0000-000000000101'),
  ('5a200000-0000-0000-0000-000000000203', '5a200000-0000-0000-0000-0000000000a0', '견본상사 방문', '5a200000-0000-0000-0000-0000000000b1', null);

insert into public.task_participant (task_id, member_id) values
  ('5a200000-0000-0000-0000-000000000202', '5a200000-0000-0000-0000-000000000005');

create function pg_temp.opportunities_seen_by(reader_user uuid)
returns text[]
language plpgsql
as $$
declare
  seen text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', json_build_object('sub', reader_user)::text, true);
  select coalesce(array_agg(name order by name), '{}') into seen from public.opportunity;
  reset role;
  return seen;
end;
$$;

select is(
  pg_temp.opportunities_seen_by('5a200000-0000-0000-0000-0000000000d3'),
  array['영업팀 딜'],
  'crm: a teammate of the owner sees the deal'
);

select is(
  pg_temp.opportunities_seen_by('5a200000-0000-0000-0000-0000000000d4'),
  array['영업팀 딜'],
  'crm: a member of a team above the owner''s sees the deal'
);

select is(
  pg_temp.opportunities_seen_by('5a200000-0000-0000-0000-0000000000d5'),
  array['담당자 없는 딜'],
  'crm: another team sees only the unowned deal it created, not the sales deal'
);

select is(
  pg_temp.opportunities_seen_by('5a200000-0000-0000-0000-0000000000d1'),
  array['담당자 없는 딜', '영업팀 딜'],
  'crm: an administrator sees every deal'
);

select lives_ok($block$do $$
declare
  organizations_seen integer;
  contacts_seen integer;
  amount_seen bigint;
  tasks_seen text[];
  rows_changed integer;
  close_refused boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"5a200000-0000-0000-0000-0000000000d6"}', true);

  select count(*) into organizations_seen from public.organization;
  assert organizations_seen = 2, 'organizations stay company-wide';

  select count(*) into contacts_seen from public.contact where email = 'sample@example.com';
  assert contacts_seen = 1, 'business contact details stay company-wide';

  select coalesce(sum(amount_minor), 0) into amount_seen from public.opportunity;
  assert amount_seen = 0, 'a total counts only the deals the reader may see';

  select array_agg(title order by title) into tasks_seen from public.task
  where company_id = '5a200000-0000-0000-0000-0000000000a0';
  assert tasks_seen = array['견본상사 방문'], 'activity on a hidden deal is hidden, activity on the organization is not';

  update public.opportunity set name = '가로챈 딜' where id = '5a200000-0000-0000-0000-000000000101';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member who cannot see a deal cannot edit it';

  begin
    perform public.crm_opportunity_close('5a200000-0000-0000-0000-000000000101', 'done', 0, now(), null, null, null);
  exception when no_data_found then
    close_refused := true;
  end;
  assert close_refused, 'a member who cannot see a deal cannot close it';
end $$;$block$, 'crm: a member outside the deal''s team reads the customer but not the deal');

select lives_ok($block$do $$
declare
  tasks_seen text[];
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"5a200000-0000-0000-0000-0000000000d5"}', true);

  select array_agg(title order by title) into tasks_seen from public.task
  where opportunity_id = '5a200000-0000-0000-0000-000000000101';
  assert tasks_seen = array['영업팀 딜 시연'], 'a participant still sees the task they take part in';
end $$;$block$, 'crm: taking part in activity on a hidden deal keeps that activity visible');

select lives_ok($block$do $$
declare
  refused_tables integer := 0;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"5a200000-0000-0000-0000-0000000000d1"}', true);

  begin
    delete from public.opportunity where id = '5a200000-0000-0000-0000-000000000102';
  exception when insufficient_privilege then
    refused_tables := refused_tables + 1;
  end;
  begin
    delete from public.contact where id = '5a200000-0000-0000-0000-0000000000c1';
  exception when insufficient_privilege then
    refused_tables := refused_tables + 1;
  end;
  begin
    delete from public.organization where id = '5a200000-0000-0000-0000-0000000000b2';
  exception when insufficient_privilege then
    refused_tables := refused_tables + 1;
  end;
  assert refused_tables = 3, 'CRM records are archived, never deleted, even by an administrator';
end $$;$block$, 'crm: nobody signed in deletes a CRM record');

select lives_ok($block$do $$
declare
  integrity_held boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"5a200000-0000-0000-0000-0000000000d6"}', true);

  begin
    update public.contact set organization_id = '5a200000-0000-0000-0000-0000000000b2'
    where id = '5a200000-0000-0000-0000-0000000000c1';
    set constraints all immediate;
  exception when check_violation then
    integrity_held := true;
  end;
  assert integrity_held, 'a contact cannot leave the organization of a deal its editor cannot see';
end $$;$block$, 'crm: record integrity holds across deals the writer cannot see');

select lives_ok($block$do $$
declare
  handed public.opportunity;
  still_seen integer;
  outsider_refused boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"5a200000-0000-0000-0000-0000000000d6"}', true);
  begin
    perform public.crm_opportunity_hand_over('5a200000-0000-0000-0000-000000000101', '5a200000-0000-0000-0000-000000000006');
  exception when no_data_found then
    outsider_refused := true;
  end;
  assert outsider_refused, 'a member who cannot see a deal cannot take it';

  perform set_config('request.jwt.claims', '{"sub":"5a200000-0000-0000-0000-0000000000d2"}', true);
  handed := public.crm_opportunity_hand_over('5a200000-0000-0000-0000-000000000101', '5a200000-0000-0000-0000-000000000005');
  assert handed.owner_id = '5a200000-0000-0000-0000-000000000005', 'the deal names its new owner';
  assert handed.updated_by = '5a200000-0000-0000-0000-000000000002', 'the hand-over is attributed to who gave it';

  select count(*) into still_seen from public.opportunity where id = '5a200000-0000-0000-0000-000000000101';
  assert still_seen = 0, 'the deal left the giver''s team with its owner';

  perform set_config('request.jwt.claims', '{"sub":"5a200000-0000-0000-0000-0000000000d5"}', true);
  select count(*) into still_seen from public.opportunity where id = '5a200000-0000-0000-0000-000000000101';
  assert still_seen = 1, 'the new owner sees the deal';
end $$;$block$, 'crm: an owner hands a deal to another team even though it leaves their sight');

select * from finish();
rollback;
