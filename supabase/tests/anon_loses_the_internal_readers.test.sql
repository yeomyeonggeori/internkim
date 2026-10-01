begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000f1', 'holder@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000f0', 'Company F', 'company-f', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000ff-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000f0', 'holder@example.test', '00000000-0000-0000-0000-0000000000f1', 'active', false);

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('000000ff-0000-0000-0000-000000000001', 'annual', true, true, -4, 'approved',
   '2026-03-02 00:00+09', '2026-03-05 23:59+09');

insert into public.attendance (member_id, kind, occurred_at) values
  ('000000ff-0000-0000-0000-000000000001', 'clock_in', '2026-03-02 09:00+09');

insert into public.contact (id, company_id, name) values
  ('000000ff-0000-0000-0000-0000000000c1', '00000000-0000-0000-0000-0000000000f0', 'A contact');

insert into public.team (id, company_id, name) values
  ('000000ff-0000-0000-0000-0000000000e1', '00000000-0000-0000-0000-0000000000f0', 'A team');

insert into public.task (id, company_id, title) values
  ('000000ff-0000-0000-0000-0000000000d1', '00000000-0000-0000-0000-0000000000f0', 'A task');

insert into public.task_participant (task_id, member_id) values
  ('000000ff-0000-0000-0000-0000000000d1', '000000ff-0000-0000-0000-000000000001');

-- Every table a policy still scoped "to public" left open to anon, swept in
-- one block: each answered rows before this migration named the role.
select lives_ok($block$do $$
declare
  seen integer;
begin
  set local role anon;
  perform set_config('request.jwt.claims', '{}', true);

  select count(*) into seen from public.leave;
  assert seen = 0, 'an anonymous read of leave answers zero rows';
  select count(*) into seen from public.attendance;
  assert seen = 0, 'an anonymous read of attendance answers zero rows';
  select count(*) into seen from public.company;
  assert seen = 0, 'an anonymous read of company answers zero rows';
  select count(*) into seen from public.member;
  assert seen = 0, 'an anonymous read of member answers zero rows';
  select count(*) into seen from public.contact;
  assert seen = 0, 'an anonymous read of contact answers zero rows';
  select count(*) into seen from public.team;
  assert seen = 0, 'an anonymous read of team answers zero rows';
  select count(*) into seen from public.task;
  assert seen = 0, 'an anonymous read of task answers zero rows';
  select count(*) into seen from public.task_participant;
  assert seen = 0, 'an anonymous read of task_participant answers zero rows';

  reset role;
end $$;$block$, 'an anonymous read of every table an internal.* policy still guarded raises nothing');

select is_empty(
  $$
  select role, name
  from (values
    ('anon', 'internal.company_of_member(uuid)'), ('anon', 'internal.company_of_task(uuid)'),
    ('anon', 'internal.company_of_team(uuid)'), ('anon', 'internal.company_of_circle(uuid)')
  ) as expected(role, name)
  where has_function_privilege(role, name, 'execute')
  $$,
  'anon holds no execute on the internal company_of_* bodies'
);

select is_empty(
  $$
  select role, name
  from (values
    ('anon', 'public.company_of_member(uuid)'), ('anon', 'public.company_of_task(uuid)'),
    ('anon', 'public.company_of_team(uuid)'), ('anon', 'public.company_of_circle(uuid)')
  ) as expected(role, name)
  where has_function_privilege(role, name, 'execute')
  $$,
  'anon holds no execute on the public company_of_* wrappers either'
);

-- The guard: a policy still scoped "to public" that names an internal.*
-- function in its using or with check runs as anon again, and turns this
-- migration's revoke back into "permission denied" the moment anyone adds
-- one. Kept on the shape rather than the list of twelve, so a new offender
-- fails this rather than going unnoticed.
select is_empty(
  $$
  select schemaname || '.' || tablename || '.' || policyname as offender
  from pg_policies
  where roles = '{public}'
    and (qual ilike '%internal.%' or with_check ilike '%internal.%')
  $$,
  'no policy still open to public names an internal.* function'
);

select * from finish();
rollback;
