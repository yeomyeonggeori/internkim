begin;
create extension if not exists pgtap with schema extensions;
select plan(22);

insert into auth.users (id, email) values
	('41000000-0000-0000-0000-000000000001', 'request-asker@example.test'),
	('41000000-0000-0000-0000-000000000002', 'request-boss@example.test'),
	('41000000-0000-0000-0000-000000000003', 'request-colleague@example.test'),
	('42000000-0000-0000-0000-000000000001', 'request-outsider@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values
	(
		'41000000-0000-0000-0000-000000000000', 'Sample Request A', 'request-a', 'KR', 'ko', 'Asia/Seoul',
		'[{"name":"Office"},{"name":"Branch"}]', '[[[],[],[],[],[],null,null]]', 480
	),
	(
		'42000000-0000-0000-0000-000000000000', 'Sample Request B', 'request-b', 'KR', 'ko', 'Asia/Seoul',
		'[{"name":"Other Office"}]', '[[[],[],[],[],[],null,null]]', 480
	);

insert into public.member (id, company_id, email, name, user_id, status, is_admin) values
	(
		'41000000-0000-0000-0000-000000000011', '41000000-0000-0000-0000-000000000000',
		'request-asker@example.test', '이샘플', '41000000-0000-0000-0000-000000000001', 'active', false
	),
	(
		'41000000-0000-0000-0000-000000000012', '41000000-0000-0000-0000-000000000000',
		'request-boss@example.test', '박예시', '41000000-0000-0000-0000-000000000002', 'active', true
	),
	(
		'41000000-0000-0000-0000-000000000013', '41000000-0000-0000-0000-000000000000',
		'request-colleague@example.test', '최견본', '41000000-0000-0000-0000-000000000003', 'active', false
	),
	(
		'42000000-0000-0000-0000-000000000011', '42000000-0000-0000-0000-000000000000',
		'request-outsider@example.test', '남의회사', '42000000-0000-0000-0000-000000000001', 'active', true
	);

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	(
		'41000000-0000-0000-0000-000000000101', '41000000-0000-0000-0000-000000000011',
		'clock_in', 'Office', '2026-08-10 09:00:00+09'
	),
	(
		'41000000-0000-0000-0000-000000000102', '41000000-0000-0000-0000-000000000011',
		'clock_in', 'Branch', '2026-08-12 09:00:00+09'
	),
	(
		'41000000-0000-0000-0000-000000000103', '41000000-0000-0000-0000-000000000011',
		'clock_out', null, '2026-08-12 18:00:00+09'
	),
	(
		'41000000-0000-0000-0000-000000000104', '41000000-0000-0000-0000-000000000011',
		'clock_in', 'Office', now() - interval '2 hours'
	);

select has_column('public', 'attendance', 'deleted_at', 'an attendance record can be marked removed');

select is(
	public.attendance_correction_window_minutes(),
	4320,
	'a member corrects their own record for three days'
);

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000011',
		'clock_out',
		((now() - interval '1 hour') at time zone 'Asia/Seoul')::date,
		((now() - interval '1 hour') at time zone 'Asia/Seoul')::time,
		null,
		'퇴근 찍는 것을 잊었습니다'
	);
	assert answer ->> 'status' = 'added', 'a record inside the window is added at once';
	assert (
		select kind = 'clock_out' and edit_reason = '퇴근 찍는 것을 잊었습니다'
		from public.attendance
		where id = (answer ->> 'eventID')::uuid
	), 'the added record carries the reason it was added for';
end $$;$block$, 'an owner adds a record inside the window');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000011',
		'clock_out',
		'2026-08-10',
		'18:00',
		null,
		'8월 10일 퇴근 누락'
	);
	assert answer ->> 'status' = 'approval_requested', 'a record this old is raised as a request';
	assert (
		select count(*) = 0
		from public.attendance
		where member_id = '41000000-0000-0000-0000-000000000011'
			and occurred_at = '2026-08-10 18:00:00+09'
	), 'nothing is written until somebody decides';
end $$;$block$, 'an owner adding an old record raises a request');

select is(
	(select count(*)::integer from public.approval where kind = 'attendance_add' and status = 'pending'),
	1,
	'the request is waiting'
);

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000003', true);
	begin
		perform public.approval_decide(
			(select id from public.approval where kind = 'attendance_add' and status = 'pending'),
			'approved',
			null
		);
	exception when insufficient_privilege then
		blocked := true;
	end;
	assert blocked, 'a colleague who is not an administrator decides nothing';
end $$;$block$, 'a colleague cannot decide a request');

select lives_ok($block$do $$
declare
	blocked boolean := false;
	request uuid;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	request := (select id from public.approval where kind = 'attendance_add' and status = 'pending');
	perform set_config('request.jwt.claim.sub', '42000000-0000-0000-0000-000000000001', true);
	begin
		perform public.approval_decide(request, 'approved', null);
	exception when insufficient_privilege then
		blocked := true;
	end;
	assert blocked, 'an administrator of another company decides nothing here';
end $$;$block$, 'an administrator of another company cannot decide a request');

select lives_ok($block$do $$
declare
	seen integer;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000003', true);
	select count(*) into seen from public.approval_pending();
	assert seen = 0, 'a colleague sees no pending request that is not theirs';

	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	select count(*) into seen from public.approval_pending();
	assert seen = 1, 'the asker sees their own pending request';

	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	select count(*) into seen from public.approval_pending();
	assert seen = 1, 'an administrator sees the request raised in their company';
	assert (select asked_by from public.approval_pending() limit 1) = '이샘플',
		'a pending request names the person who raised it';
end $$;$block$, 'a pending request is visible to its asker and to an administrator');

select lives_ok($block$do $$
declare
	answer jsonb;
	request uuid;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	request := (select id from public.approval where kind = 'attendance_add' and status = 'pending');
	answer := public.approval_decide(request, 'approved', '확인했습니다');
	assert answer ->> 'status' = 'approved', 'the decision is reported back';
	assert (
		select kind = 'clock_out' and edit_reason = '8월 10일 퇴근 누락'
		from public.attendance
		where id = ((answer -> 'applied') ->> 'eventID')::uuid
	), 'approving writes the record the asker wanted';
	assert (
		select status = 'approved' and applied_at is not null
			and decided_by = '41000000-0000-0000-0000-000000000012'
			and decision_note = '확인했습니다'
		from public.approval where id = request
	), 'the ledger records who decided and that it was carried out';
end $$;$block$, 'an administrator approves and the record is written');

select lives_ok($block$do $$
declare
	blocked boolean := false;
	request uuid;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	request := (select id from public.approval where kind = 'attendance_add');
	begin
		perform public.approval_decide(request, 'rejected', null);
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'a settled request is not decided a second time';
	assert (select status from public.approval where id = request) = 'approved',
		'the first decision stands';
end $$;$block$, 'a request is decided once');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_remove(
		'41000000-0000-0000-0000-000000000104',
		'잘못 찍었습니다'
	);
	assert answer ->> 'status' = 'removed', 'a record inside the window is removed at once';
	assert (
		select count(*) = 0 from public.attendance
		where id = '41000000-0000-0000-0000-000000000104'
	), 'a removed record is no longer readable';
end $$;$block$, 'an owner removes a record inside the window');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_remove(
		'41000000-0000-0000-0000-000000000101',
		'그날은 출근하지 않았습니다'
	);
	assert answer ->> 'status' = 'approval_requested', 'removing a record this old is raised as a request';
	assert (
		select count(*) = 1 from public.attendance
		where id = '41000000-0000-0000-0000-000000000101'
	), 'the record stays until somebody decides';
end $$;$block$, 'an owner removing an old record raises a request');

select lives_ok($block$do $$
declare
	request uuid;
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	request := (select id from public.approval where kind = 'attendance_remove' and status = 'pending');
	answer := public.approval_decide(request, 'rejected', '그날 출근 기록이 맞습니다');
	assert answer ->> 'status' = 'rejected', 'the refusal is reported back';
	assert answer -> 'applied' = 'null'::jsonb, 'a refusal carries out nothing';
	assert (
		select count(*) = 1 from public.attendance
		where id = '41000000-0000-0000-0000-000000000101'
	), 'the record the request would have removed is still there';
	assert (select applied_at is null from public.approval where id = request),
		'a refused request was never applied';
end $$;$block$, 'an administrator refuses and nothing is carried out');

select lives_ok($block$do $$
declare
	answer jsonb;
	request uuid;
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_remove(
		'41000000-0000-0000-0000-000000000102',
		'출근을 잘못 찍었습니다'
	);
	request := (answer ->> 'approvalID')::uuid;

	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	perform public.attendance_remove('41000000-0000-0000-0000-000000000102', '관리자가 먼저 지웠습니다');

	begin
		perform public.approval_decide(request, 'approved', null);
	exception when no_data_found then
		blocked := true;
	end;
	assert blocked, 'a request whose target is gone cannot be carried out';
	assert (select status from public.approval where id = request) = 'pending',
		'a decision that could not be carried out leaves the request pending';
end $$;$block$, 'a decision rolls back when the payload no longer applies');

select lives_ok($block$do $$
declare
	answer jsonb;
	request uuid;
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000011',
		'clock_in',
		'2026-08-11',
		'09:00',
		'Office',
		'8월 11일 출근 누락'
	);
	request := (answer ->> 'approvalID')::uuid;

	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000003', true);
	begin
		perform public.approval_withdraw(request);
	exception when insufficient_privilege then
		blocked := true;
	end;
	assert blocked, 'only the person who raised a request withdraws it';

	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	perform public.approval_withdraw(request);
	assert (select status from public.approval where id = request) = 'withdrawn',
		'the asker withdraws their own request';
end $$;$block$, 'a request is withdrawn only by the person who raised it');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000013',
		'clock_in',
		'2026-08-03',
		'09:00',
		'Office',
		'관리자가 대신 기록합니다'
	);
	assert answer ->> 'status' = 'added', 'an administrator writes an old record for somebody else at once';
end $$;$block$, 'an administrator adds a record for another member with no window');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	begin
		perform public.attendance_add(
			'41000000-0000-0000-0000-000000000013',
			'clock_in',
			'2026-08-04',
			'09:00',
			'Office',
			'남의 기록을 만들어 봅니다'
		);
	exception when insufficient_privilege then
		blocked := true;
	end;
	assert blocked, 'a member writes nothing into somebody else record';
end $$;$block$, 'a member cannot add a record for a colleague');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	begin
		perform public.attendance_add(
			'41000000-0000-0000-0000-000000000011',
			'clock_out',
			((now() + interval '1 day') at time zone 'Asia/Seoul')::date,
			'09:00',
			null,
			'미래를 기록해 봅니다'
		);
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'nobody records attendance that has not happened';
end $$;$block$, 'a record cannot be added in the future');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	begin
		perform public.attendance_add(
			'41000000-0000-0000-0000-000000000011',
			'clock_out',
			'2026-08-12',
			'17:00',
			null,
			'퇴근 앞에 퇴근을 넣어 봅니다'
		);
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'a clock out cannot be placed in front of another clock out';
end $$;$block$, 'a backdated record answers to the records around it');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	begin
		perform public.attendance_add(
			'41000000-0000-0000-0000-000000000011',
			'clock_out',
			'2026-08-10',
			'20:00',
			null,
			'   '
		);
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'a record is not added without saying why';
end $$;$block$, 'adding a record needs a reason');

select ok(
	not has_function_privilege(
		'authenticated',
		to_regprocedure('public.approval_apply(uuid)'),
		'execute'
	),
	'nobody carries out a request except through a decision'
);

select ok(
	has_function_privilege(
		'authenticated',
		to_regprocedure('public.approval_decide(uuid,text,text)'),
		'execute'
	),
	'a signed-in member may reach the decision'
);

select finish();
rollback;
