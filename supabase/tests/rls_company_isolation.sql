begin;

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-0000000000a1', 'a@example.test'),
  ('00000000-0000-0000-0000-0000000000b1', 'b@example.test'),
  ('00000000-0000-0000-0000-0000000000c1', 'invited@example.test'),
  ('00000000-0000-0000-0000-0000000000a9', 'admin@example.test');

insert into public.company (id, name) values
  ('00000000-0000-0000-0000-0000000000a0', 'Company A'),
  ('00000000-0000-0000-0000-0000000000b0', 'Company B');

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('000000aa-0000-0000-0000-000000000000', '00000000-0000-0000-0000-0000000000a0', 'admin@example.test', '00000000-0000-0000-0000-0000000000a9', 'active', true);

insert into public.member (id, company_id, email, user_id, status) values
  ('000000aa-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000a0', 'a@example.test', '00000000-0000-0000-0000-0000000000a1', 'active'),
  ('000000aa-0000-0000-0000-000000000002', '00000000-0000-0000-0000-0000000000a0', 'invited@example.test', '00000000-0000-0000-0000-0000000000c1', 'invited'),
  ('000000bb-0000-0000-0000-000000000001', '00000000-0000-0000-0000-0000000000b0', 'b@example.test', '00000000-0000-0000-0000-0000000000b1', 'active');

insert into public.member (id, company_id, email) values
  ('000000aa-0000-0000-0000-000000000003', '00000000-0000-0000-0000-0000000000a0', 'imported@example.test');

insert into public.attendance (member_id, kind) values
  ('000000aa-0000-0000-0000-000000000001', 'clock_in'),
  ('000000aa-0000-0000-0000-000000000002', 'clock_in'),
  ('000000bb-0000-0000-0000-000000000001', 'clock_in');

insert into public.credential (member_id, kind, external_id) values
  ('000000aa-0000-0000-0000-000000000002', 'buzz', 'pubkey-unclaimed'),
  ('000000bb-0000-0000-0000-000000000001', 'buzz', 'pubkey-b');

do $$
declare
  visible_attendance integer;
  visible_companies integer;
  visible_members integer;
  visible_credentials integer;
  impersonation_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  select count(*) into visible_companies from public.company;
  assert visible_companies = 1, 'a member must not see another company';

  select count(*) into visible_members from public.member;
  assert visible_members = 4, 'a member must see colleagues including pending ones, and nobody else';

  select count(*) into visible_attendance from public.attendance;
  assert visible_attendance = 2, 'attendance is visible company-wide, including an unclaimed member''s';

  select count(*) into visible_credentials from public.credential;
  assert visible_credentials = 1, 'only the same company messenger credentials are visible';

  begin
    insert into public.attendance (member_id, kind)
    values ('000000aa-0000-0000-0000-000000000002', 'clock_in');
  exception when insufficient_privilege then
    impersonation_blocked := true;
  end;
  assert impersonation_blocked, 'writing attendance for another member must be rejected';

  reset role;
  raise notice 'rls_company_isolation: all assertions passed';
end $$;

do $$
declare
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  update public.company set name = 'Renamed by non-admin'
    where id = '00000000-0000-0000-0000-0000000000a0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a non-admin must not be able to rename their company';

  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a9"}', true);

  update public.company set name = 'Renamed by admin'
    where id = '00000000-0000-0000-0000-0000000000a0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 1, 'an admin must be able to rename their own company';

  update public.company set name = 'Renamed across companies'
    where id = '00000000-0000-0000-0000-0000000000b0';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'an admin must not be able to rename another company';

  reset role;
  raise notice 'admin: company writes are admin-only and company-scoped';
end $$;

do $$
declare
  claimed_member uuid;
  rows_changed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000a1"}', true);

  update public.member set is_admin = true where id = '000000aa-0000-0000-0000-000000000001';
  get diagnostics rows_changed = row_count;
  assert rows_changed = 0, 'a member must not be able to make themselves an admin';

  reset role;
  raise notice 'escalation: member rows are not client-writable';
end $$;

do $$
declare
  invited_member uuid;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"00000000-0000-0000-0000-0000000000c1"}', true);

  select public.my_member() into invited_member;
  assert invited_member = '000000aa-0000-0000-0000-000000000002',
    'an invited member who has never signed in is already bound to their auth user';

  reset role;
  raise notice 'invite: an invited member resolves without a separate claim step';
end $$;

do $$
declare
  projected_email text;
  bound_user uuid;
  kept_status public.member_status;
begin
  update auth.users set email = 'renamed@example.test'
    where id = '00000000-0000-0000-0000-0000000000c1';

  select email, user_id, status into projected_email, bound_user, kept_status
    from public.member where id = '000000aa-0000-0000-0000-000000000002';

  assert projected_email = 'renamed@example.test',
    'changing one account address must update that member';
  assert bound_user = '00000000-0000-0000-0000-0000000000c1',
    'changing an address must not move the account binding';
  assert kept_status = 'invited',
    'changing an address must not change member status';

  raise notice 'email change: one member follows their account address';
end $$;

do $$
declare
  moved_members integer;
begin
  update auth.users
    set email = replace(email, '@example.test', '@example.co')
    where id in (
      select user_id from public.member
      where company_id = '00000000-0000-0000-0000-0000000000a0' and user_id is not null
    );

  select count(*) into moved_members
    from public.member
    where company_id = '00000000-0000-0000-0000-0000000000a0'
      and email like '%@example.co';
  assert moved_members = 3, 'a company-wide domain change must follow every bound member';

  select count(*) into moved_members
    from public.member
    where company_id = '00000000-0000-0000-0000-0000000000a0' and user_id is null;
  assert moved_members = 1, 'a pending member has no account address to follow';

  raise notice 'email change: a company-wide domain move follows every bound member';
end $$;

do $$
declare
  offboarded_status public.member_status;
  surviving_attendance integer;
begin
  update public.member set status = 'departed'
    where id = '000000aa-0000-0000-0000-000000000001';
  delete from auth.users where id = '00000000-0000-0000-0000-0000000000a1';

  select status into offboarded_status
    from public.member where id = '000000aa-0000-0000-0000-000000000001';
  assert offboarded_status = 'departed',
    'an offboarded member must stay departed, not be relabelled withdrawn';

  select count(*) into surviving_attendance
    from public.attendance where member_id = '000000aa-0000-0000-0000-000000000001';
  assert surviving_attendance = 1, 'offboarding keeps the attendance record';

  raise notice 'offboarding: an explicit departure survives account deletion';
end $$;

do $$
declare
  bound_user uuid;
begin
  insert into auth.users (id, email)
    values ('00000000-0000-0000-0000-0000000000d1', 'imported@example.test');

  select user_id into bound_user
    from public.member where id = '000000aa-0000-0000-0000-000000000003';
  assert bound_user = '00000000-0000-0000-0000-0000000000d1',
    'creating an auth user must bind a pending member with the same email';

  raise notice 'import: a pending member binds to its account on sign-up';
end $$;

do $$
declare
  surviving_member integer;
  surviving_attendance integer;
  detached_user_id uuid;
  withdrawn_status public.member_status;
begin
  delete from auth.users where id = '00000000-0000-0000-0000-0000000000b1';

  select status into withdrawn_status
    from public.member where id = '000000bb-0000-0000-0000-000000000001';
  assert withdrawn_status = 'withdrawn',
    'an account deleted without an explicit departure must be recorded as withdrawn';

  select count(*) into surviving_member
    from public.member where id = '000000bb-0000-0000-0000-000000000001';
  assert surviving_member = 1, 'offboarding must not remove the member record';

  select user_id into detached_user_id
    from public.member where id = '000000bb-0000-0000-0000-000000000001';
  assert detached_user_id is null, 'a departed member must lose its sign-in binding';

  select count(*) into surviving_attendance
    from public.attendance where member_id = '000000bb-0000-0000-0000-000000000001';
  assert surviving_attendance = 1, 'attendance stays with the company after the account is deleted';

  raise notice 'withdrawal: a self-deleted account keeps the company record';
end $$;

rollback;
