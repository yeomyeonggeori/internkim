begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

insert into auth.users (id, email) values
	('41000000-0000-0000-0000-000000000001', 'record-owner@example.test'),
	('41000000-0000-0000-0000-000000000002', 'record-boss@example.test'),
	('41000000-0000-0000-0000-000000000003', 'record-colleague@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values (
	'41000000-0000-0000-0000-000000000000', 'Sample Record', 'record-a', 'KR', 'ko', 'Asia/Seoul',
	'[{"name":"Office"},{"name":"Branch"}]', '[[[],[],[],[],[],null,null]]', 480
);

insert into public.member (id, company_id, email, name, user_id, status, is_admin) values
	(
		'41000000-0000-0000-0000-000000000011', '41000000-0000-0000-0000-000000000000',
		'record-owner@example.test', '이샘플', '41000000-0000-0000-0000-000000000001', 'active', false
	),
	(
		'41000000-0000-0000-0000-000000000012', '41000000-0000-0000-0000-000000000000',
		'record-boss@example.test', '박예시', '41000000-0000-0000-0000-000000000002', 'active', true
	),
	(
		'41000000-0000-0000-0000-000000000013', '41000000-0000-0000-0000-000000000000',
		'record-colleague@example.test', '최견본', '41000000-0000-0000-0000-000000000003', 'active', false
	);

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	(
		'41000000-0000-0000-0000-000000000101', '41000000-0000-0000-0000-000000000011',
		'clock_in', 'Office', '2026-08-10 09:00:00+09'
	),
	(
		'41000000-0000-0000-0000-000000000102', '41000000-0000-0000-0000-000000000011',
		'clock_in', 'Branch', now() - interval '2 hours'
	);

select hasnt_table('public', 'approval', 'nothing about attendance waits on a stored decision');

select is(
	public.attendance_backdated_after_minutes(),
	4320,
	'reaching back more than three days is an administrator write'
);

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000011', 'clock_out', null, null, null, null
	);
	assert answer ->> 'status' = 'added', 'clocking out is written at once';
	assert (answer ->> 'backdated')::boolean = false, 'a clock made now is not backdated';
	assert (
		select edit_reason is null and occurred_at <= now()
		from public.attendance where id = (answer ->> 'eventID')::uuid
	), 'a clock nobody wrote by hand carries no reason';
end $$;$block$, 'omitting the day and the time clocks the moment it is called');

select lives_ok($block$do $$
declare
	answer jsonb;
	kept text;
	written uuid;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000011', 'clock_out', current_date - 1, '18:00'::time, null, null
	);
	assert answer ->> 'status' = 'added', 'a record written by hand needs no reason';
	written := (answer ->> 'eventID')::uuid;
	select edit_reason into kept from public.attendance where id = written;
	assert kept is null, 'a reason nobody gave is not invented';

	reset role;
	delete from public.attendance where id = written;
end $$;$block$, 'a hand-written record is taken without a reason');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000011', 'clock_out', '2026-08-10'::date, '18:00'::time,
		null, '8월 10일 퇴근 누락'
	);
	assert answer ->> 'status' = 'asked', 'an old record is handed to an administrator';
	assert answer ->> 'eventID' is null, 'nothing is written by the person who cannot write it';
	assert (
		select count(*) = 0
		from public.attendance
		where member_id = '41000000-0000-0000-0000-000000000011'
			and occurred_at = '2026-08-10 18:00:00+09'
	), 'the record waits for the administrator who may write it';
end $$;$block$, 'an owner reaching back more than three days asks an administrator');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);
	answer := public.attendance_add(
		'41000000-0000-0000-0000-000000000013', 'clock_in', '2026-08-11'::date, '09:00'::time,
		'Office', '대신 기록합니다'
	);
	assert answer ->> 'status' = 'added', 'an administrator writes anybody record';
end $$;$block$, 'an administrator writes a colleague record');

select lives_ok($block$do $$
declare
	refused boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000003', true);
	begin
		perform public.attendance_add(
			'41000000-0000-0000-0000-000000000011', 'clock_in', '2026-08-13'::date, '09:00'::time,
			'Office', '남의 기록'
		);
	exception when insufficient_privilege then
		refused := true;
	end;
	assert refused, 'a colleague who is not an administrator writes nobody else record';
end $$;$block$, 'a colleague cannot write somebody else record');

select lives_ok($block$do $$
declare
	refused boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	begin
		perform public.attendance_add(
			'41000000-0000-0000-0000-000000000011', 'clock_in',
			((now() + interval '1 day') at time zone 'Asia/Seoul')::date, '09:00'::time,
			'Office', '내일 출근'
		);
	exception when check_violation then
		refused := true;
	end;
	assert refused, 'nobody records a moment that has not happened';
end $$;$block$, 'a record in the future is refused');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_remove(
		'41000000-0000-0000-0000-000000000101', '두 번 찍혔습니다'
	);
	assert answer ->> 'status' = 'asked', 'removing an old record is handed to an administrator';
end $$;$block$, 'an owner reaching back more than three days asks an administrator to remove it');

set local role postgres;

select is(
	(select deleted_at is null from public.attendance where id = '41000000-0000-0000-0000-000000000101'),
	true,
	'the record the person could not remove is still there'
);

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_correct(
		jsonb_build_array(jsonb_build_object(
			'event_id', '41000000-0000-0000-0000-000000000102',
			'local_date', ((now() - interval '3 hours') at time zone 'Asia/Seoul')::date,
			'local_time', ((now() - interval '3 hours') at time zone 'Asia/Seoul')::time,
			'location', 'Office'
		)),
		'장소를 잘못 골랐습니다'
	);
	assert answer ->> 'status' = 'corrected', 'a recent record is corrected at once';
	assert (answer ->> 'backdated')::boolean = false, 'correcting a recent record tells nobody';
end $$;$block$, 'an owner corrects a recent record quietly');

select lives_ok($block$do $$
declare
	blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000003', true);
	begin
		perform public.attendance_remove('41000000-0000-0000-0000-000000000102', '남의 기록');
	exception when insufficient_privilege then
		blocked := true;
	end;
	assert blocked, 'a colleague removes nobody else record';
end $$;$block$, 'a colleague cannot remove somebody else record');

select is(
	(select count(*)::integer from public.attendance where member_id = '41000000-0000-0000-0000-000000000011' and deleted_at is null),
	3,
	'the owner is left with the records they were allowed to write'
);

select * from finish();
rollback;
