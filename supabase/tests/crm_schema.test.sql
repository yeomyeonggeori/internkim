begin;
create extension if not exists pgtap with schema extensions;
select plan(18);

select has_table('public', 'organization', 'crm: organization table exists');
select has_table('public', 'opportunity', 'crm: opportunity table exists');
select has_column('public', 'company', 'crm_vocabulary', 'crm: company stores its vocabulary');
select has_column('public', 'company', 'currency_code', 'crm: company stores its base currency');
select has_column('public', 'contact', 'organization_id', 'crm: contact can belong to an organization');
select has_column('public', 'task', 'organization_id', 'crm: task can link an organization');
select has_column('public', 'task', 'opportunity_id', 'crm: task can link an opportunity');
select has_column('public', 'task', 'contact_id', 'crm: task can link an external contact');
select hasnt_column(
  'public',
  'task_participant',
  'contact_id',
  'crm: task participants remain internal members only'
);

insert into auth.users (id, email) values
  ('59400000-0000-0000-0000-000000000001', 'crm-a@example.test'),
  ('59400000-0000-0000-0000-000000000002', 'crm-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('59400000-0000-0000-0000-0000000000a0', 'CRM Company A', 'crm-company-a', 'KR', 'ko', 'Asia/Seoul'),
  ('59400000-0000-0000-0000-0000000000b0', 'CRM Company B', 'crm-company-b', 'US', 'en-US', 'America/New_York');

insert into public.member (id, company_id, email, user_id, status) values
  ('59400000-0000-0000-0000-0000000000a1', '59400000-0000-0000-0000-0000000000a0', 'crm-a@example.test', '59400000-0000-0000-0000-000000000001', 'active'),
  ('59400000-0000-0000-0000-0000000000b1', '59400000-0000-0000-0000-0000000000b0', 'crm-b@example.test', '59400000-0000-0000-0000-000000000002', 'active');

insert into public.organization (id, company_id, name) values
  ('59400000-0000-0000-0000-000000000101', '59400000-0000-0000-0000-0000000000a0', 'Sample Organization A'),
  ('59400000-0000-0000-0000-000000000102', '59400000-0000-0000-0000-0000000000a0', 'Sample Organization A2'),
  ('59400000-0000-0000-0000-000000000201', '59400000-0000-0000-0000-0000000000b0', 'Sample Organization B');

insert into public.contact (id, company_id, organization_id, name, email) values
  ('59400000-0000-0000-0000-000000000111', '59400000-0000-0000-0000-0000000000a0', '59400000-0000-0000-0000-000000000101', '이샘플', 'sample-a@example.test'),
  ('59400000-0000-0000-0000-000000000112', '59400000-0000-0000-0000-0000000000a0', '59400000-0000-0000-0000-000000000102', '박예시', 'sample-a2@example.test'),
  ('59400000-0000-0000-0000-000000000211', '59400000-0000-0000-0000-0000000000b0', '59400000-0000-0000-0000-000000000201', '최견본', 'sample-b@example.test');

insert into public.opportunity (
  id,
  company_id,
  organization_id,
  contact_id,
  name,
  pipeline_id,
  stage_id,
  amount_minor,
  currency_code
) values
  (
    '59400000-0000-0000-0000-000000000121',
    '59400000-0000-0000-0000-0000000000a0',
    '59400000-0000-0000-0000-000000000101',
    '59400000-0000-0000-0000-000000000111',
    'Sample Opportunity A1',
    'partnership',
    'review',
    18000000,
    'KRW'
  ),
  (
    '59400000-0000-0000-0000-000000000122',
    '59400000-0000-0000-0000-0000000000a0',
    '59400000-0000-0000-0000-000000000101',
    '59400000-0000-0000-0000-000000000111',
    'Sample Opportunity A2',
    'sponsorship',
    'proposal',
    8500000,
    'KRW'
  );

set constraints all immediate;

select lives_ok(
  $$insert into public.task (company_id, title) values ('59400000-0000-0000-0000-0000000000a0', 'General task')$$,
  'crm: a general task needs no CRM link'
);

select lives_ok(
  $$
    insert into public.task (
      company_id,
      title,
      organization_id,
      opportunity_id,
      contact_id
    ) values (
      '59400000-0000-0000-0000-0000000000a0',
      'Connected task',
      '59400000-0000-0000-0000-000000000101',
      '59400000-0000-0000-0000-000000000121',
      '59400000-0000-0000-0000-000000000111'
    )
  $$,
  'crm: one contact can serve multiple opportunities and a connected task'
);

select throws_ok(
  $$
    insert into public.contact (company_id, organization_id, name)
    values (
      '59400000-0000-0000-0000-0000000000a0',
      '59400000-0000-0000-0000-000000000201',
      'Cross-company contact'
    )
  $$,
  '23503',
  null,
  'crm: a contact cannot cross the company boundary'
);

select throws_ok(
  $$
    insert into public.opportunity (
      company_id,
      organization_id,
      contact_id,
      name,
      pipeline_id,
      stage_id
    ) values (
      '59400000-0000-0000-0000-0000000000a0',
      '59400000-0000-0000-0000-000000000101',
      '59400000-0000-0000-0000-000000000112',
      'Mismatched opportunity',
      'partnership',
      'review'
    )
  $$,
  '23514',
  null,
  'crm: an opportunity contact must belong to its organization'
);

select throws_ok(
  $$
    insert into public.task (company_id, title, organization_id, opportunity_id)
    values (
      '59400000-0000-0000-0000-0000000000a0',
      'Mismatched opportunity task',
      '59400000-0000-0000-0000-000000000102',
      '59400000-0000-0000-0000-000000000121'
    )
  $$,
  '23514',
  null,
  'crm: a task opportunity must belong to its organization'
);

select throws_ok(
  $$
    insert into public.task (company_id, title, organization_id, contact_id)
    values (
      '59400000-0000-0000-0000-0000000000a0',
      'Mismatched contact task',
      '59400000-0000-0000-0000-000000000101',
      '59400000-0000-0000-0000-000000000112'
    )
  $$,
  '23514',
  null,
  'crm: a task contact must belong to its organization'
);

select lives_ok($block$do $$
declare
  actor uuid;
  visible_count integer;
begin
  set local role authenticated;
  perform set_config(
    'request.jwt.claims',
    '{"sub":"59400000-0000-0000-0000-000000000001","role":"authenticated"}',
    true
  );

  insert into public.organization (id, company_id, name)
  values (
    '59400000-0000-0000-0000-000000000103',
    '59400000-0000-0000-0000-0000000000a0',
    'Audited Organization'
  );

  select created_by into actor
  from public.organization
  where id = '59400000-0000-0000-0000-000000000103';
  assert actor = '59400000-0000-0000-0000-0000000000a1',
    'the authenticated member is recorded as creator';

  select count(*) into visible_count from public.organization;
  assert visible_count = 3, 'company A sees only its three organizations';

  reset role;
end $$;$block$, 'crm: RLS isolates companies and audit fields record the actor');

select lives_ok($block$do $$
declare
  organization_privileges integer;
  opportunity_privileges integer;
begin
  select count(*) into organization_privileges
  from information_schema.role_table_grants
  where table_schema = 'public'
    and table_name = 'organization'
    and grantee = 'authenticated'
    and privilege_type in ('SELECT', 'INSERT', 'UPDATE', 'DELETE');

  select count(*) into opportunity_privileges
  from information_schema.role_table_grants
  where table_schema = 'public'
    and table_name = 'opportunity'
    and grantee = 'authenticated'
    and privilege_type in ('SELECT', 'INSERT', 'UPDATE', 'DELETE');

  assert organization_privileges = 4, 'organization has all API grants';
  assert opportunity_privileges = 4, 'opportunity has all API grants';
end $$;$block$, 'crm: new tables expose the expected API grants');

select lives_ok(
  $$
    insert into public.task (company_id, title, organization_id)
    values (
      '59400000-0000-0000-0000-0000000000a0',
      'Organization-only task',
      '59400000-0000-0000-0000-000000000101'
    )
  $$,
  'crm: a task may link only an organization'
);

select * from finish();
rollback;
