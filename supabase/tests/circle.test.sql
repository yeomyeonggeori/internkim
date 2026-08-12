begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
  ('44000000-0000-0000-0000-000000000001', 'circle-a@example.test'),
  ('44000000-0000-0000-0000-000000000002', 'circle-b@example.test'),
  ('44000000-0000-0000-0000-000000000003', 'circle-admin@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('44000000-0000-0000-0000-0000000000a0', 'Circle A', 'circle-a', 'KR', 'ko', 'Asia/Seoul'),
  ('44000000-0000-0000-0000-0000000000b0', 'Circle B', 'circle-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('44000000-0000-0000-0000-0000000000a1', '44000000-0000-0000-0000-0000000000a0',
   'circle-a@example.test', '44000000-0000-0000-0000-000000000001', 'active', false),
  ('44000000-0000-0000-0000-0000000000a2', '44000000-0000-0000-0000-0000000000a0',
   'circle-admin@example.test', '44000000-0000-0000-0000-000000000003', 'active', true),
  ('44000000-0000-0000-0000-0000000000b1', '44000000-0000-0000-0000-0000000000b0',
   'circle-b@example.test', '44000000-0000-0000-0000-000000000002', 'active', false);

insert into public.circle (id, company_id, name) values
  ('44000000-0000-0000-0000-0000000000c1', '44000000-0000-0000-0000-0000000000a0', 'Staff'),
  ('44000000-0000-0000-0000-0000000000c2', '44000000-0000-0000-0000-0000000000a0', 'C-Level'),
  ('44000000-0000-0000-0000-0000000000d1', '44000000-0000-0000-0000-0000000000b0', 'Staff');

insert into public.circle_member (circle_id, member_id) values
  ('44000000-0000-0000-0000-0000000000c1', '44000000-0000-0000-0000-0000000000a1');

select throws_ok(
  $$insert into public.circle (company_id, name)
    values ('44000000-0000-0000-0000-0000000000a0', 'Staff')$$,
  '23505',
  null,
  'one company does not get two circles of the same name, the way it does not get two teams'
);

select lives_ok(
  $$insert into public.circle (company_id, name)
    values ('44000000-0000-0000-0000-0000000000b0', 'C-Level')$$,
  'the same name in another company is a different circle'
);

do $$
declare
  visible_circles integer;
  visible_memberships integer;
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000001"}', true);

  select count(*) into visible_circles from public.circle;
  assert visible_circles = 2, 'a member sees the circles of their own company and no other';

  select count(*) into visible_memberships from public.circle_member;
  assert visible_memberships = 1, 'a member sees memberships of their own company only';

  update public.circle set name = 'Renamed' where name = 'Staff';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member who is not an admin does not edit a circle';

  assert public.is_in_circle('44000000-0000-0000-0000-0000000000c1'), 'a member is in the circle they were put in';
  assert not public.is_in_circle('44000000-0000-0000-0000-0000000000c2'), 'a member is not in a circle they were not';
  assert not public.is_in_circle(null), 'a path with no circle in it resolves to no membership';

  reset role;
end;
$$;

select pass('a member reads their own company circles and memberships, and no other');
select pass('a member who is not an admin cannot edit a circle');
select pass('is_in_circle answers for the caller, and denies a null circle');

do $$
declare
  rows_changed integer;
  planting_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"44000000-0000-0000-0000-000000000003"}', true);

  update public.circle set name = 'Renamed' where name = 'Staff'
    and company_id = '44000000-0000-0000-0000-0000000000a0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'an admin edits a circle of their own company';

  begin
    insert into public.circle (company_id, name)
    values ('44000000-0000-0000-0000-0000000000b0', 'Planted');
  exception when insufficient_privilege then
    planting_blocked := true;
  end;
  assert planting_blocked, 'an admin of one company does not create a circle in another';

  reset role;
end;
$$;

select pass('an admin keeps the circles of their own company, and no other');

select finish();
rollback;
