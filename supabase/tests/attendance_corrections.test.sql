begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

insert into auth.users (id, email) values
	('31000000-0000-0000-0000-000000000001', 'correction-owner@example.test'),
	('31000000-0000-0000-0000-000000000002', 'correction-admin@example.test'),
	('32000000-0000-0000-0000-000000000001', 'correction-outsider@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values
	(
		'31000000-0000-0000-0000-000000000000', 'Sample Correction A', 'correction-a', 'KR', 'ko', 'Asia/Seoul',
		'[{"name":"Office"},{"name":"Branch"}]', '[[[],[],[],[],[],null,null]]', 480
	),
	(
		'32000000-0000-0000-0000-000000000000', 'Sample Correction B', 'correction-b', 'KR', 'ko', 'Asia/Seoul',
		'[{"name":"Other Office"}]', '[[[],[],[],[],[],null,null]]', 480
	);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
	(
		'31000000-0000-0000-0000-000000000011', '31000000-0000-0000-0000-000000000000',
		'correction-owner@example.test', '31000000-0000-0000-0000-000000000001', 'active', false
	),
	(
		'31000000-0000-0000-0000-000000000012', '31000000-0000-0000-0000-000000000000',
		'correction-admin@example.test', '31000000-0000-0000-0000-000000000002', 'active', true
	),
	(
		'31000000-0000-0000-0000-000000000013', '31000000-0000-0000-0000-000000000000',
		'correction-colleague@example.test', null, 'active', false
	),
	(
		'32000000-0000-0000-0000-000000000011', '32000000-0000-0000-0000-000000000000',
		'correction-outsider@example.test', '32000000-0000-0000-0000-000000000001', 'active', true
	);

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	(
		'31000000-0000-0000-0000-000000000101', '31000000-0000-0000-0000-000000000011',
		'clock_in', 'Office', '2026-08-10 09:00:00+09'
	),
	(
		'31000000-0000-0000-0000-000000000102', '31000000-0000-0000-0000-000000000013',
		'clock_in', 'Office', '2026-08-10 09:10:00+09'
	),
	(
		'32000000-0000-0000-0000-000000000101', '32000000-0000-0000-0000-000000000011',
		'clock_in', 'Other Office', '2026-08-10 09:20:00+09'
	);

select has_column(
	'public',
	'attendance',
	'original_occurred_at',
	'attendance preserves the first time before a correction'
);

select has_column(
	'public',
	'attendance',
	'edit_reason',
	'attendance stores the latest correction reason'
);

select lives_ok($block$do $$
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000001', true);

	perform public.correct_attendance_events(
		'[{"event_id":"31000000-0000-0000-0000-000000000101","local_date":"2026-08-10","local_time":"09:30","location":"Branch"}]'::jsonb,
		'출근 시간 정정'
	);
	assert (
		select original_occurred_at = '2026-08-10 09:00:00+09'
			and occurred_at = '2026-08-10 09:30:00+09'
			and location = 'Branch'
			and edit_reason = '출근 시간 정정'
		from public.attendance
		where id = '31000000-0000-0000-0000-000000000101'
	), 'the owner correction must preserve the first time and store the new values';

	perform public.correct_attendance_events(
		'[{"event_id":"31000000-0000-0000-0000-000000000101","local_date":"2026-08-10","local_time":"09:45","location":"Office"}]'::jsonb,
		'재확인 후 수정'
	);
	assert (
		select original_occurred_at = '2026-08-10 09:00:00+09'
			and occurred_at = '2026-08-10 09:45:00+09'
			and edit_reason = '재확인 후 수정'
		from public.attendance
		where id = '31000000-0000-0000-0000-000000000101'
	), 'a later correction must retain the first time and replace the reason';
end $$;$block$, 'an owner can correct their own attendance twice');

select lives_ok($block$do $$
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000002', true);
	perform public.correct_attendance_events(
		'[{"event_id":"31000000-0000-0000-0000-000000000102","local_date":"2026-08-10","local_time":"09:25","location":"Branch"}]'::jsonb,
		'관리자 확인'
	);
	assert (
		select occurred_at = '2026-08-10 09:25:00+09' and edit_reason = '관리자 확인'
		from public.attendance
		where id = '31000000-0000-0000-0000-000000000102'
	), 'a same-company admin must be able to correct a colleague event';
end $$;$block$, 'a same-company admin can correct a colleague attendance event');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000001', true);
	begin
		perform public.correct_attendance_events(
			'[{"event_id":"31000000-0000-0000-0000-000000000102","local_date":"2026-08-10","local_time":"09:40","location":"Office"}]'::jsonb,
			'권한 없는 수정'
		);
	exception when insufficient_privilege then
		blocked := true;
	end;
	assert blocked, 'a non-admin member must not correct a colleague event';
end $$;$block$, 'a non-admin member cannot correct a colleague attendance event');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000002', true);
	begin
		perform public.correct_attendance_events(
			'[{"event_id":"31000000-0000-0000-0000-000000000101","local_date":"2026-08-10","local_time":"10:00","location":"Office"},{"event_id":"32000000-0000-0000-0000-000000000101","local_date":"2026-08-10","local_time":"10:00","location":"Other Office"}]'::jsonb,
			'일괄 수정'
		);
	exception when insufficient_privilege then
		blocked := true;
	end;
	assert blocked, 'a batch containing another company event must be rejected';
	assert (
		select occurred_at = '2026-08-10 09:45:00+09'
		from public.attendance
		where id = '31000000-0000-0000-0000-000000000101'
	), 'a rejected batch must not partially update an allowed event';
end $$;$block$, 'attendance correction batches are atomic across authorization failures');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000001', true);
	begin
		perform public.correct_attendance_events(
			'[{"event_id":"31000000-0000-0000-0000-000000000101","local_date":"2026-08-10","local_time":"10:00","location":"Office"}]'::jsonb,
			'   '
		);
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'a blank reason must be rejected';
end $$;$block$, 'attendance corrections require a nonblank reason');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000001', true);
	begin
		perform public.correct_attendance_events(
			'[{"event_id":"31000000-0000-0000-0000-000000000101","local_date":"2026-08-10","local_time":"10:00","location":"Unknown"}]'::jsonb,
			'장소 수정'
		);
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'an unregistered location must be rejected';
end $$;$block$, 'attendance corrections keep registered location validation');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000001', true);
	begin
		perform public.correct_attendance_events(
			'[{"event_id":"31000000-0000-0000-0000-000000000101","local_date":"2999-01-01","local_time":"09:00","location":"Office"}]'::jsonb,
			'시간 수정'
		);
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'a future occurrence time must be rejected';
end $$;$block$, 'attendance corrections reject future times');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '31000000-0000-0000-0000-000000000001', true);
	begin
		update public.attendance
		set edit_reason = '사유만 변경'
		where id = '31000000-0000-0000-0000-000000000101';
	exception when check_violation then
		blocked := true;
	end;
	assert blocked, 'a correction reason must not change without a time or location correction';
end $$;$block$, 'attendance correction reasons cannot be overwritten independently');

select * from finish();
rollback;
