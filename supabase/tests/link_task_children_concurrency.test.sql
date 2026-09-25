create extension if not exists pgtap with schema extensions;
create extension if not exists dblink with schema extensions;

delete from public.task_participant
where member_id = '4d000000-0000-0000-0000-0000000000a1';
delete from public.task
where company_id = '4d000000-0000-0000-0000-0000000000a0';
delete from public.member
where id = '4d000000-0000-0000-0000-0000000000a1';
delete from public.company
where id = '4d000000-0000-0000-0000-0000000000a0';
delete from auth.users
where id = '4d000000-0000-0000-0000-000000000001';
do $$
begin
	if exists (
		select 1
		from pg_catalog.pg_roles
		where rolname = 'task_child_dblink_login'
	) then
		revoke usage on schema public from task_child_dblink_login;
		revoke execute on function public.task_children_link(uuid, uuid[])
			from task_child_dblink_login;
		revoke execute on function public.task_parent_set(uuid, uuid)
			from task_child_dblink_login;
		drop role task_child_dblink_login;
	end if;
end;
$$;

create temp table task_child_dblink_password (value text not null);
insert into task_child_dblink_password values (gen_random_uuid()::text);

do $$
declare
	login_password text;
begin
	select value into login_password
	from task_child_dblink_password;
	execute format(
		'create role task_child_dblink_login login password %L',
		login_password
	);
end;
$$;

grant usage on schema public to task_child_dblink_login;
grant execute on function public.task_children_link(uuid, uuid[])
	to task_child_dblink_login;
grant execute on function public.task_parent_set(uuid, uuid)
	to task_child_dblink_login;

insert into auth.users (id, email) values
	('4d000000-0000-0000-0000-000000000001', 'race-link@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
	('4d000000-0000-0000-0000-0000000000a0', 'Race Link', 'race-link', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
	(
		'4d000000-0000-0000-0000-0000000000a1',
		'4d000000-0000-0000-0000-0000000000a0',
		'race-link@example.test',
		'4d000000-0000-0000-0000-000000000001',
		'active'
	);

insert into public.task (id, company_id, title, parent_task_id) values
	('4d000000-0000-0000-0000-000000000101', '4d000000-0000-0000-0000-0000000000a0', 'Winning parent', null),
	('4d000000-0000-0000-0000-000000000102', '4d000000-0000-0000-0000-0000000000a0', 'Challenging parent', null),
	('4d000000-0000-0000-0000-000000000103', '4d000000-0000-0000-0000-0000000000a0', 'Contended child', null);

insert into public.task_participant (task_id, member_id) values
	('4d000000-0000-0000-0000-000000000101', '4d000000-0000-0000-0000-0000000000a1'),
	('4d000000-0000-0000-0000-000000000102', '4d000000-0000-0000-0000-0000000000a1'),
	('4d000000-0000-0000-0000-000000000103', '4d000000-0000-0000-0000-0000000000a1');

begin;
select plan(2);

create temp table task_child_link_test_connection (connection_string text not null);
insert into task_child_link_test_connection
select format(
	'host=%s port=%s dbname=postgres user=task_child_dblink_login password=%s',
	host(inet_server_addr()),
	inet_server_port(),
	value
)
from task_child_dblink_password;

do $$
declare
	connection_string text;
begin
	select task_child_link_test_connection.connection_string
	into connection_string
	from task_child_link_test_connection;

	perform extensions.dblink_connect('task_child_holder', connection_string);
	perform extensions.dblink_connect('task_child_challenger', connection_string);

	perform extensions.dblink_exec('task_child_holder', $session$
		do $configure$
		begin
			perform set_config('request.jwt.claim.sub', '4d000000-0000-0000-0000-000000000001', false);
		end;
		$configure$;
	$session$);
	perform extensions.dblink_exec('task_child_challenger', $session$
		do $configure$
		begin
			perform set_config('request.jwt.claim.sub', '4d000000-0000-0000-0000-000000000001', false);
		end;
		$configure$;
	$session$);

	perform extensions.dblink_exec('task_child_holder', $holder$
		begin;
		do $link$
		begin
			perform public.task_parent_set(
				'4d000000-0000-0000-0000-000000000103',
				'4d000000-0000-0000-0000-000000000101'
			);
		end;
		$link$;
	$holder$);
end;
$$;

create temp table task_child_challenger_backend (pid integer not null);
insert into task_child_challenger_backend
select pid
from extensions.dblink(
	'task_child_challenger',
	'select pg_backend_pid()'
) as backend(pid integer);

do $$
begin
	perform extensions.dblink_send_query('task_child_challenger', $challenger$
		select public.task_children_link(
			'4d000000-0000-0000-0000-000000000102',
			array['4d000000-0000-0000-0000-000000000103'::uuid]
		);
	$challenger$);

	for attempt in 1..100 loop
		exit when exists (
			select 1
			from pg_catalog.pg_stat_activity
			join task_child_challenger_backend
				on task_child_challenger_backend.pid = pg_stat_activity.pid
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
		join task_child_challenger_backend
			on task_child_challenger_backend.pid = pg_stat_activity.pid
		where pg_stat_activity.wait_event_type = 'Lock'
	),
	'link task children: a competing request reaches the child row lock'
);

do $$
begin
	perform extensions.dblink_exec('task_child_holder', 'commit');
	perform *
	from extensions.dblink_get_result(
		'task_child_challenger',
		false
	) as result(value text);
end;
$$;

select is(
	(
		select parent_task_id
		from public.task
		where id = '4d000000-0000-0000-0000-000000000103'
	),
	'4d000000-0000-0000-0000-000000000101'::uuid,
	'link task children: a concurrent request cannot overwrite the first committed parent'
);

do $$
begin
	perform extensions.dblink_disconnect('task_child_holder');
	perform extensions.dblink_disconnect('task_child_challenger');
end;
$$;

select * from finish();
rollback;

delete from public.task_participant
where member_id = '4d000000-0000-0000-0000-0000000000a1';
delete from public.task
where company_id = '4d000000-0000-0000-0000-0000000000a0';
delete from public.member
where id = '4d000000-0000-0000-0000-0000000000a1';
delete from public.company
where id = '4d000000-0000-0000-0000-0000000000a0';
delete from auth.users
where id = '4d000000-0000-0000-0000-000000000001';
revoke usage on schema public from task_child_dblink_login;
revoke execute on function public.task_children_link(uuid, uuid[])
	from task_child_dblink_login;
revoke execute on function public.task_parent_set(uuid, uuid)
	from task_child_dblink_login;
drop role task_child_dblink_login;
