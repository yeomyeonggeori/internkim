begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
  ('c4000000-0000-0000-0000-000000000001', 'replace-admin@example.test'),
  ('c4000000-0000-0000-0000-000000000002', 'replace-member@example.test'),
  ('c4000000-0000-0000-0000-000000000003', 'replace-colleague@example.test'),
  ('c4000000-0000-0000-0000-000000000004', 'replace-autonomous@example.test');

insert into public.company (id, name, slug, country, locale, timezone, rules) values
  (
    'c4000000-0000-0000-0000-0000000000a0', 'Replace', 'replace', 'KR', 'ko', 'Asia/Seoul',
    '{"attendanceWorkPolicy": {"revisions": [{"workMode": "fixed"}]}}'::jsonb
  ),
  (
    'c4000000-0000-0000-0000-0000000000b0', 'Replace Autonomous', 'replace-autonomous', 'KR', 'ko', 'Asia/Seoul',
    '{"attendanceWorkPolicy": {"revisions": [{"workMode": "autonomous"}]}}'::jsonb
  );

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('c4000000-0000-0000-0000-0000000000a1', 'c4000000-0000-0000-0000-0000000000a0',
   'replace-admin@example.test', 'c4000000-0000-0000-0000-000000000001', 'active', true),
  ('c4000000-0000-0000-0000-0000000000a2', 'c4000000-0000-0000-0000-0000000000a0',
   'replace-member@example.test', 'c4000000-0000-0000-0000-000000000002', 'active', false),
  ('c4000000-0000-0000-0000-0000000000a3', 'c4000000-0000-0000-0000-0000000000a0',
   'replace-colleague@example.test', 'c4000000-0000-0000-0000-000000000003', 'active', false),
  ('c4000000-0000-0000-0000-0000000000b1', 'c4000000-0000-0000-0000-0000000000b0',
   'replace-autonomous@example.test', 'c4000000-0000-0000-0000-000000000004', 'active', false);

insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
  ('c4000000-0000-0000-0000-0000000000e1', 'c4000000-0000-0000-0000-0000000000a2',
   'annual', true, true, -1, 'approved', '2099-03-02 00:00+09', '2099-03-02 23:59+09'),
  ('c4000000-0000-0000-0000-0000000000e2', 'c4000000-0000-0000-0000-0000000000a2',
   'annual', true, true, -1, 'approved', '2099-03-03 00:00+09', '2099-03-03 23:59+09'),
  ('c4000000-0000-0000-0000-0000000000e3', 'c4000000-0000-0000-0000-0000000000a2',
   'annual', true, true, -1, 'approved', '2020-03-02 00:00+09', '2020-03-02 23:59+09'),
  ('c4000000-0000-0000-0000-0000000000e4', 'c4000000-0000-0000-0000-0000000000a3',
   'annual', true, true, -1, 'approved', '2099-03-02 00:00+09', '2099-03-02 23:59+09'),
  ('c4000000-0000-0000-0000-0000000000e5', 'c4000000-0000-0000-0000-0000000000b1',
   'annual', true, true, -1, 'approved', '2099-03-02 00:00+09', '2099-03-02 23:59+09'),
  ('c4000000-0000-0000-0000-0000000000e6', 'c4000000-0000-0000-0000-0000000000a2',
   'annual', true, true, -1, 'approved', '2099-03-04 00:00+09', '2099-03-04 23:59+09');

select lives_ok($block$do $$
declare
  replacement public.leave;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000002"}', true);

  insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, original_leave_id) values
    ('c4000000-0000-0000-0000-0000000000f1', 'c4000000-0000-0000-0000-0000000000a2',
     'annual', true, true, -1, 'requested', '2099-03-09 00:00+09', '2099-03-09 23:59+09',
     'c4000000-0000-0000-0000-0000000000e1');

  reset role;
  select * into replacement from public.leave where id = 'c4000000-0000-0000-0000-0000000000f1';
  assert replacement.status = 'requested', 'the replacement waits for a decision';
  assert (select status from public.leave where id = 'c4000000-0000-0000-0000-0000000000e1') = 'approved',
    'the approved leave stands while its replacement waits';
end $$;$block$, 'a person asks for different days for an approved leave of their own');

select lives_ok($block$do $$
declare
  second_blocked boolean := false;
  foreign_blocked boolean := false;
  started_blocked boolean := false;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000002"}', true);

  begin
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, original_leave_id) values
      ('c4000000-0000-0000-0000-0000000000a2', 'annual', true, true, -1, 'requested',
       '2099-03-10 00:00+09', '2099-03-10 23:59+09', 'c4000000-0000-0000-0000-0000000000e1');
  exception when unique_violation then
    second_blocked := true;
  end;
  assert second_blocked, 'one approved leave has one replacement waiting at a time';

  begin
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, original_leave_id) values
      ('c4000000-0000-0000-0000-0000000000a2', 'annual', true, true, -1, 'requested',
       '2099-03-10 00:00+09', '2099-03-10 23:59+09', 'c4000000-0000-0000-0000-0000000000e4');
  exception when invalid_parameter_value then
    foreign_blocked := true;
  end;
  assert foreign_blocked, 'a leave must not replace somebody else''s';

  begin
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, original_leave_id) values
      ('c4000000-0000-0000-0000-0000000000a2', 'annual', true, true, -1, 'requested',
       '2099-03-10 00:00+09', '2099-03-10 23:59+09', 'c4000000-0000-0000-0000-0000000000e3');
  exception when invalid_parameter_value then
    started_blocked := true;
  end;
  assert started_blocked, 'a leave that has started is not replaced by its owner';

  reset role;
end $$;$block$, 'a replacement names one approved, unstarted leave of the same person');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000001"}', true);

  update public.leave set status = 'approved' where id = 'c4000000-0000-0000-0000-0000000000f1';

  reset role;
  assert not exists (select 1 from public.leave where id = 'c4000000-0000-0000-0000-0000000000e1'),
    'approving the replacement removes the leave it replaces';
  assert (select original_leave_id is null and status = 'approved'
          from public.leave where id = 'c4000000-0000-0000-0000-0000000000f1'),
    'the approved replacement is an ordinary approved leave';
end $$;$block$, 'approving a replacement removes its original');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000002"}', true);
  insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, original_leave_id) values
    ('c4000000-0000-0000-0000-0000000000f2', 'c4000000-0000-0000-0000-0000000000a2',
     'annual', true, true, -1, 'requested', '2099-03-11 00:00+09', '2099-03-11 23:59+09',
     'c4000000-0000-0000-0000-0000000000e2');

  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000001"}', true);
  update public.leave set status = 'rejected' where id = 'c4000000-0000-0000-0000-0000000000f2';

  reset role;
  assert (select status from public.leave where id = 'c4000000-0000-0000-0000-0000000000e2') = 'approved',
    'rejecting the replacement leaves the original approved';
  assert (select original_leave_id from public.leave where id = 'c4000000-0000-0000-0000-0000000000f2')
    = 'c4000000-0000-0000-0000-0000000000e2', 'a rejected replacement still names what it asked to replace';
end $$;$block$, 'rejecting a replacement keeps its original');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000004"}', true);
  insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, original_leave_id) values
    ('c4000000-0000-0000-0000-0000000000f3', 'c4000000-0000-0000-0000-0000000000b1',
     'annual', true, true, -1, 'requested', '2099-03-09 00:00+09', '2099-03-09 23:59+09',
     'c4000000-0000-0000-0000-0000000000e5');

  reset role;
  assert (select status from public.leave where id = 'c4000000-0000-0000-0000-0000000000f3') = 'approved',
    'a replacement under autonomous work is taken without a decision';
  assert not exists (select 1 from public.leave where id = 'c4000000-0000-0000-0000-0000000000e5'),
    'and its original is removed at once';
end $$;$block$, 'autonomous work replaces a leave without asking');

select lives_ok($block$do $$
declare
  rows_removed integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000002"}', true);

  delete from public.leave where id = 'c4000000-0000-0000-0000-0000000000e3';
  get diagnostics rows_removed = row_count;
  assert rows_removed = 0, 'a leave that has started stays closed to its owner';

  delete from public.leave
    where member_id = 'c4000000-0000-0000-0000-0000000000a2' and days >= 0;
  get diagnostics rows_removed = row_count;
  assert rows_removed = 0, 'a grant stays closed to the person it was given to';

  delete from public.leave where id = 'c4000000-0000-0000-0000-0000000000e4';
  get diagnostics rows_removed = row_count;
  assert rows_removed = 0, 'a colleague''s leave stays closed';

  delete from public.leave where id = 'c4000000-0000-0000-0000-0000000000e6';
  get diagnostics rows_removed = row_count;
  assert rows_removed = 1, 'a person takes back their own approved leave before it starts';

  reset role;
end $$;$block$, 'a person takes back an approved leave that has not started');

select lives_ok($block$do $$
declare
  over_blocked boolean := false;
begin
  update public.company
    set rules = rules || jsonb_build_object('attendanceLeavePolicy', jsonb_build_object(
      'leaveTypes', jsonb_build_array(jsonb_build_object('id', 'family-care', 'usageLimitMilliDays', 1000))
    ))
    where id = 'c4000000-0000-0000-0000-0000000000a0';

  insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
    ('c4000000-0000-0000-0000-0000000000e7', 'c4000000-0000-0000-0000-0000000000a3',
     'family-care', false, false, -1, 'approved', '2099-04-01 00:00+09', '2099-04-01 23:59+09');

  insert into public.leave (id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, original_leave_id) values
    ('c4000000-0000-0000-0000-0000000000f4', 'c4000000-0000-0000-0000-0000000000a3',
     'family-care', false, false, -1, 'requested', '2099-04-08 00:00+09', '2099-04-08 23:59+09',
     'c4000000-0000-0000-0000-0000000000e7');

  begin
    insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at) values
      ('c4000000-0000-0000-0000-0000000000a3', 'family-care', false, false, -1, 'requested',
       '2099-04-15 00:00+09', '2099-04-15 23:59+09');
  exception when invalid_parameter_value then
    over_blocked := true;
  end;
  assert over_blocked, 'a leave that replaces nothing still counts against the limit';
end $$;$block$, 'a replacement is not counted on top of the leave it replaces');

select lives_ok($block$do $$
declare
  named uuid;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"c4000000-0000-0000-0000-000000000003"}', true);
  select original_leave_id into named from public.leave_in_full()
    where id = 'c4000000-0000-0000-0000-0000000000f4';
  reset role;
  assert named = 'c4000000-0000-0000-0000-0000000000e7', 'the full read names the leave a request replaces';
end $$;$block$, 'the full read says what a leave replaces');

select * from finish();
rollback;
