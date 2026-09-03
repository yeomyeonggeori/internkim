begin;
create extension if not exists pgtap with schema extensions;
select plan(3);

delete from public.company;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000f1', 'holder@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('00000000-0000-0000-0000-0000000000f0', 'Company F', 'company-f', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000ff-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000f0', 'holder@example.test', '00000000-0000-0000-0000-0000000000f1', 'active', false);

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('000000ff-0000-0000-0000-000000000001', '연차', true, true, 4, 'approved',
   '2026-03-02 00:00+09', '2026-03-05 23:59+09');

insert into public.attendance (member_id, kind, occurred_at) values
  ('000000ff-0000-0000-0000-000000000001', 'clock_in', '2026-03-02 09:00+09');

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
  reset role;
end $$;$block$, 'an anonymous read of leave and attendance raises nothing');

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

select * from finish();
rollback;
