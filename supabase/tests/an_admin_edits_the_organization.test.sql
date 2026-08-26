begin;
create extension if not exists pgtap with schema extensions;
select plan(6);

insert into auth.users (id, email) values
	('41000000-0000-0000-0000-000000000001', 'organization-admin@example.com'),
	('41000000-0000-0000-0000-000000000002', 'organization-member@example.com'),
	('42000000-0000-0000-0000-000000000001', 'organization-outsider@example.com');

insert into public.company (id, name, slug, country, locale, timezone) values
	('41000000-0000-0000-0000-000000000000', 'Sample Organization A', 'organization-a', 'KR', 'ko', 'Asia/Seoul'),
	('42000000-0000-0000-0000-000000000000', 'Sample Organization B', 'organization-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, name, email, user_id, status, is_admin) values
	(
		'41000000-0000-0000-0000-000000000011', '41000000-0000-0000-0000-000000000000',
		'이샘플', 'organization-admin@example.com', '41000000-0000-0000-0000-000000000001', 'active', true
	),
	(
		'41000000-0000-0000-0000-000000000012', '41000000-0000-0000-0000-000000000000',
		'박예시', 'organization-member@example.com', '41000000-0000-0000-0000-000000000002', 'active', false
	),
	(
		'41000000-0000-0000-0000-000000000013', '41000000-0000-0000-0000-000000000000',
		'최견본', 'organization-colleague@example.com', null, 'active', false
	),
	(
		'42000000-0000-0000-0000-000000000011', '42000000-0000-0000-0000-000000000000',
		'이샘플', 'organization-outsider@example.com', '42000000-0000-0000-0000-000000000001', 'active', true
	);

select lives_ok($block$do $$
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	perform public.team_save($teams$[
		{"id": "41000000-0000-0000-0000-0000000000a1", "name": "개발팀"},
		{"id": "41000000-0000-0000-0000-0000000000a2", "name": "플랫폼", "parentID": "41000000-0000-0000-0000-0000000000a1"}
	]$teams$::jsonb);

	assert (
		select count(*) from public.team where company_id = '41000000-0000-0000-0000-000000000000'
	) = 2, 'an administrator writes the organizations the browser sent';
	assert (
		select position from public.team where id = '41000000-0000-0000-0000-0000000000a2'
	) = 1, 'an organization takes the position it was given in';
	assert (
		select parent_team_id from public.team where id = '41000000-0000-0000-0000-0000000000a2'
	) = '41000000-0000-0000-0000-0000000000a1', 'a child organization keeps the parent it was given';
end $$;$block$, 'an administrator saves the organization tree');

select lives_ok($block$do $$
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	perform public.member_profiles_save($profiles$[
		{
			"memberID": "41000000-0000-0000-0000-000000000012",
			"jobTitle": "  프론트엔드 개발자  ",
			"groupID": "41000000-0000-0000-0000-0000000000a2",
			"hireDate": "2026-03-12",
			"phoneNumber": "010-0000-0000",
			"supervisorID": "41000000-0000-0000-0000-000000000011"
		}
	]$profiles$::jsonb);

	assert (
		select job_title = '프론트엔드 개발자'
			and team_id = '41000000-0000-0000-0000-0000000000a2'
			and joined_at = date '2026-03-12'
			and phone_number = '010-0000-0000'
			and supervisor_id = '41000000-0000-0000-0000-000000000011'
		from public.member where id = '41000000-0000-0000-0000-000000000012'
	), 'an administrator saves every organization attribute of a colleague';

	perform public.member_profiles_save($cleared$[
		{
			"memberID": "41000000-0000-0000-0000-000000000012",
			"jobTitle": "",
			"groupID": "",
			"hireDate": "",
			"phoneNumber": "",
			"supervisorID": ""
		}
	]$cleared$::jsonb);

	assert (
		select job_title is null and team_id is null and joined_at is null
			and phone_number is null and supervisor_id is null
		from public.member where id = '41000000-0000-0000-0000-000000000012'
	), 'an emptied field clears the attribute instead of storing an empty string';
end $$;$block$, 'an administrator saves a member organization profile');

select lives_ok($block$do $$
declare
	profile_save_blocked boolean := false;
	team_save_blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);

	begin
		perform public.member_profiles_save($profiles$[
			{"memberID": "41000000-0000-0000-0000-000000000013", "jobTitle": "대표"}
		]$profiles$::jsonb);
	exception when raise_exception then
		profile_save_blocked := true;
	end;
	assert profile_save_blocked, 'a member who is not an administrator cannot edit a colleague';

	begin
		perform public.team_save($teams$[
			{"id": "41000000-0000-0000-0000-0000000000a1", "name": "개발팀"}
		]$teams$::jsonb);
	exception when raise_exception then
		team_save_blocked := true;
	end;
	assert team_save_blocked, 'a member who is not an administrator cannot edit the organization tree';
end $$;$block$, 'only an administrator edits the organization');

select lives_ok($block$do $$
declare
	foreign_member_blocked boolean := false;
	foreign_team_blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '42000000-0000-0000-0000-000000000001', true);

	begin
		perform public.member_profiles_save($profiles$[
			{"memberID": "41000000-0000-0000-0000-000000000013", "jobTitle": "대표"}
		]$profiles$::jsonb);
	exception when raise_exception then
		foreign_member_blocked := true;
	end;
	assert foreign_member_blocked, 'an administrator cannot edit a member of another company';

	begin
		perform public.team_save($teams$[
			{"id": "41000000-0000-0000-0000-0000000000a1", "name": "빼앗은 조직"}
		]$teams$::jsonb);
	exception when raise_exception then
		foreign_team_blocked := true;
	end;
	assert foreign_team_blocked, 'an administrator cannot claim an organization of another company';
end $$;$block$, 'another company cannot reach this organization');

select lives_ok($block$do $$
declare
	supervisor_loop_blocked boolean := false;
	team_loop_blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	assert (
		select name from public.team where id = '41000000-0000-0000-0000-0000000000a1'
	) = '개발팀', 'a write refused from another company left the organization untouched';

	begin
		perform public.member_profiles_save($profiles$[
			{
				"memberID": "41000000-0000-0000-0000-000000000011",
				"supervisorID": "41000000-0000-0000-0000-000000000013"
			},
			{
				"memberID": "41000000-0000-0000-0000-000000000013",
				"supervisorID": "41000000-0000-0000-0000-000000000011"
			}
		]$profiles$::jsonb);
	exception when raise_exception then
		supervisor_loop_blocked := true;
	end;
	assert supervisor_loop_blocked, 'a supervisor chain that loops back is refused';

	begin
		perform public.team_save($teams$[
			{"id": "41000000-0000-0000-0000-0000000000a1", "name": "개발팀", "parentID": "41000000-0000-0000-0000-0000000000a2"},
			{"id": "41000000-0000-0000-0000-0000000000a2", "name": "플랫폼", "parentID": "41000000-0000-0000-0000-0000000000a1"}
		]$teams$::jsonb);
	exception when raise_exception then
		team_loop_blocked := true;
	end;
	assert team_loop_blocked, 'an organization that contains itself is refused';
end $$;$block$, 'a loop in the organization is refused');

select lives_ok($block$do $$
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	perform public.member_profiles_save($profiles$[
		{"memberID": "41000000-0000-0000-0000-000000000013", "groupID": "41000000-0000-0000-0000-0000000000a2"}
	]$profiles$::jsonb);

	perform public.team_save($teams$[
		{"id": "41000000-0000-0000-0000-0000000000a1", "name": "개발팀"}
	]$teams$::jsonb);

	assert (
		select count(*) from public.team where company_id = '41000000-0000-0000-0000-000000000000'
	) = 2, 'an organization the payload leaves out stays where it was';
	assert (
		select team_id from public.member where id = '41000000-0000-0000-0000-000000000013'
	) = '41000000-0000-0000-0000-0000000000a2', 'a member keeps the organization the payload never mentioned';
	assert (
		select parent_team_id is null from public.team where id = '41000000-0000-0000-0000-0000000000a1'
	), 'an organization named in the payload takes the parent the payload gives it';
end $$;$block$, 'an organization left out of the payload is left alone');

select * from finish();
rollback;
