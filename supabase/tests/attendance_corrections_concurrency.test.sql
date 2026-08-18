create extension if not exists pgtap with schema extensions;
create extension if not exists dblink with schema extensions;

delete from public.attendance
where member_id = '45000000-0000-0000-0000-0000000000a1';
delete from public.member
where id = '45000000-0000-0000-0000-0000000000a1';
delete from public.company
where id = '45000000-0000-0000-0000-0000000000a0';
delete from auth.users
where id = '45000000-0000-0000-0000-000000000001';
do $$
begin
	if exists (
		select 1
		from pg_catalog.pg_roles
		where rolname = 'attendance_correction_dblink_login'
	) then
		revoke usage on schema public from attendance_correction_dblink_login;
		revoke execute on function public.correct_attendance_events(jsonb, text)
			from attendance_correction_dblink_login;
		drop role attendance_correction_dblink_login;
	end if;
end;
$$;

create temp table attendance_correction_dblink_password (value text not null);
insert into attendance_correction_dblink_password values (gen_random_uuid()::text);

do $$
declare
	login_password text;
begin
	select value into login_password
	from attendance_correction_dblink_password;
	execute format(
		'create role attendance_correction_dblink_login login password %L',
		login_password
	);
end;
$$;

grant usage on schema public to attendance_correction_dblink_login;
grant execute on function public.correct_attendance_events(jsonb, text)
	to attendance_correction_dblink_login;

insert into auth.users (id, email) values
	('45000000-0000-0000-0000-000000000001', 'attendance-race@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
	(
		'45000000-0000-0000-0000-0000000000a0',
		'샘플회사',
		'attendance-race',
		'KR',
		'ko',
		'Asia/Seoul',
		'[{"name":"Office"}]'
	);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
	(
		'45000000-0000-0000-0000-0000000000a1',
		'45000000-0000-0000-0000-0000000000a0',
		'attendance-race@example.test',
		'45000000-0000-0000-0000-000000000001',
		'active',
		true
	);

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	(
		'45000000-0000-0000-0000-000000000101',
		'45000000-0000-0000-0000-0000000000a1',
		'clock_in',
		'Office',
		'2026-08-10 09:00:00+09'
	),
	(
		'45000000-0000-0000-0000-000000000102',
		'45000000-0000-0000-0000-0000000000a1',
		'clock_out',
		null,
		'2026-08-10 10:00:00+09'
	);

begin;
select plan(2);

create temp table attendance_correction_test_connection (connection_string text not null);
insert into attendance_correction_test_connection
select format(
	'host=%s port=%s dbname=postgres user=attendance_correction_dblink_login password=%s',
	host(inet_server_addr()),
	inet_server_port(),
	value
)
from attendance_correction_dblink_password;

do $$
declare
	connection_string text;
begin
	select attendance_correction_test_connection.connection_string
	into connection_string
	from attendance_correction_test_connection;

	perform extensions.dblink_connect('attendance_correction_holder', connection_string);
	perform extensions.dblink_connect('attendance_correction_challenger', connection_string);

	perform extensions.dblink_exec('attendance_correction_holder', $session$
		do $configure$
		begin
			perform set_config('request.jwt.claim.sub', '45000000-0000-0000-0000-000000000001', false);
		end;
		$configure$;
	$session$);
	perform extensions.dblink_exec('attendance_correction_challenger', $session$
		do $configure$
		begin
			perform set_config('request.jwt.claim.sub', '45000000-0000-0000-0000-000000000001', false);
		end;
		$configure$;
	$session$);

	perform extensions.dblink_exec('attendance_correction_holder', $holder$
		begin;
		do $correct$
		begin
			perform public.correct_attendance_events(
				'[{"event_id":"45000000-0000-0000-0000-000000000101","local_date":"2026-08-10","local_time":"09:50","location":"Office"}]'::jsonb,
				'첫 번째 수정'
			);
		end;
		$correct$;
	$holder$);
end;
$$;

create temp table attendance_correction_challenger_backend (pid integer not null);
insert into attendance_correction_challenger_backend
select pid
from extensions.dblink(
	'attendance_correction_challenger',
	'select pg_backend_pid()'
) as backend(pid integer);

do $$
begin
	perform extensions.dblink_send_query('attendance_correction_challenger', $challenger$
		select public.correct_attendance_events(
			'[{"event_id":"45000000-0000-0000-0000-000000000102","local_date":"2026-08-10","local_time":"09:20","location":"Office"}]'::jsonb,
			'두 번째 수정'
		);
	$challenger$);

	for attempt in 1..100 loop
		exit when exists (
			select 1
			from pg_catalog.pg_stat_activity
			join attendance_correction_challenger_backend
				on attendance_correction_challenger_backend.pid = pg_stat_activity.pid
			where pg_stat_activity.wait_event_type = 'Lock'
		);
		perform pg_catalog.pg_sleep(0.01);
	end loop;
end;
$$;

select ok(
	exists (
		select 1
		from pg_catalog.pg_stat_activity
		join attendance_correction_challenger_backend
			on attendance_correction_challenger_backend.pid = pg_stat_activity.pid
		where pg_stat_activity.wait_event_type = 'Lock'
	),
	'attendance correction: a competing request waits for the member event lock'
);

do $$
begin
	perform extensions.dblink_exec('attendance_correction_holder', 'commit');
	perform *
	from extensions.dblink_get_result(
		'attendance_correction_challenger',
		false
	) as result(value text);
end;
$$;

select is(
	(
		select array_agg(occurred_at order by id)
		from public.attendance
		where member_id = '45000000-0000-0000-0000-0000000000a1'
	),
	array[
		'2026-08-10 09:50:00+09'::timestamptz,
		'2026-08-10 10:00:00+09'::timestamptz
	],
	'attendance correction: the waiting request rechecks order after the first commit'
);

do $$
begin
	perform extensions.dblink_disconnect('attendance_correction_holder');
	perform extensions.dblink_disconnect('attendance_correction_challenger');
end;
$$;

select * from finish();
rollback;

delete from public.attendance
where member_id = '45000000-0000-0000-0000-0000000000a1';
delete from public.member
where id = '45000000-0000-0000-0000-0000000000a1';
delete from public.company
where id = '45000000-0000-0000-0000-0000000000a0';
delete from auth.users
where id = '45000000-0000-0000-0000-000000000001';
revoke usage on schema public from attendance_correction_dblink_login;
revoke execute on function public.correct_attendance_events(jsonb, text)
	from attendance_correction_dblink_login;
drop role attendance_correction_dblink_login;
