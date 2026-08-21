begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000a1', 'a@example.test'),
  ('00000000-0000-0000-0000-0000000000b1', 'b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000a0', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('00000000-0000-0000-0000-0000000000b0', 'Company B', 'company-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('000000aa-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'a@example.test', '00000000-0000-0000-0000-0000000000a1', 'active'),
  ('000000bb-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000b0', 'b@example.test', '00000000-0000-0000-0000-0000000000b1', 'active');

insert into public.organization (id, company_id, name) values
  ('000000a0-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', '우리 관계처'),
  ('000000a0-0000-0000-0000-000000000009', '00000000-0000-0000-0000-0000000000b0', '남의 관계처');

insert into public.contact (id, company_id, organization_id, name) values
  ('000000c0-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', '000000a0-0000-0000-0000-000000000001', '이샘플'),
  ('000000c0-0000-0000-0000-000000000009', '00000000-0000-0000-0000-0000000000b0', '000000a0-0000-0000-0000-000000000009', '박예시');

insert into public.opportunity (id, company_id, organization_id, name, pipeline_id, stage_id) values
  ('0000000a-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', '000000a0-0000-0000-0000-000000000001', '우리 진행 건', 'sales', 'waiting'),
  ('0000000a-0000-0000-0000-000000000009', '00000000-0000-0000-0000-0000000000b0', '000000a0-0000-0000-0000-000000000009', '남의 진행 건', 'sales', 'waiting');

select lives_ok($block$do $$
declare
  visible_organizations integer;
  visible_contacts integer;
  visible_opportunities integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  select count(*) into visible_organizations from public.organization;
  assert visible_organizations = 1, 'an organization from another company must be invisible';

  select count(*) into visible_contacts from public.contact;
  assert visible_contacts = 1, 'a contact from another company must be invisible';

  select count(*) into visible_opportunities from public.opportunity;
  assert visible_opportunities = 1, 'an opportunity from another company must be invisible';
end $$;$block$, 'crm: a member reads only the CRM records of their own company');

select lives_ok($block$do $$
declare
  rows_changed integer;
  planted_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  update public.opportunity set name = '가로챈 진행 건'
    where company_id = '00000000-0000-0000-0000-0000000000b0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member must not edit another company opportunity';

  begin
    insert into public.organization (company_id, name)
    values ('00000000-0000-0000-0000-0000000000b0', '심어둔 관계처');
  exception when insufficient_privilege then
    planted_blocked := true;
  end;
  assert planted_blocked, 'a member must not plant a record in another company';
end $$;$block$, 'crm: a member cannot write into another company');

select lives_ok($block$do $$
declare
  archived_still_stored integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  update public.organization
  set archived_at = now(), archived_by = '000000aa-0000-0000-0000-000000000001'
  where id = '000000a0-0000-0000-0000-000000000001';

  select count(*) into archived_still_stored
  from public.organization
  where id = '000000a0-0000-0000-0000-000000000001' and archived_at is not null;
  assert archived_still_stored = 1, 'archiving keeps the record and only stamps when it was archived';
end $$;$block$, 'crm: archiving marks a record rather than deleting it');

select * from finish();
rollback;
