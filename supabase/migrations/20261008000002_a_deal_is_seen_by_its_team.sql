-- Every member of a company read and wrote every opportunity, amount
-- included, and could delete any organization, contact or opportunity outright
-- through the REST API while the product only ever archives them. Salesforce,
-- HubSpot and Pipedrive all scope deal records by owner and team and widen from
-- there, so an opportunity is now seen by the member responsible for it (its
-- owner, or whoever created it while nobody owns it), by that member's team and
-- every team above it, and by administrators. Organizations and contacts stay
-- company-wide: several teams deal with the same customer.
--
-- The rule lives in row level security alone, so the screens, the public API,
-- the agent's tools, the reports and the activity list all follow it without a
-- second copy. Activity is a task on an opportunity, so a task on a deal the
-- reader cannot see is hidden unless they take part in it or asked for it.
--
-- Two functions had to move with it. crm_opportunity_close was security definer
-- and checked only the company, so it closed deals its caller could not see; it
-- now runs as the caller. enforce_crm_reference_consistency ran as the caller,
-- so it would have stopped seeing the opportunities and tasks it keeps
-- consistent; integrity is not access, so it now runs as its owner.

create function internal.opportunity_visible_to_caller(target_owner uuid, target_creator uuid)
returns boolean
language sql
security definer
stable
set search_path = public
as $$
  select public.is_company_admin()
    or public.my_member() in (target_owner, target_creator)
    or exists (
      with recursive responsible_team_line(team_id, depth) as (
        select member.team_id, 0
        from public.member
        where member.id = coalesce(target_owner, target_creator)
          and member.team_id is not null
        union all
        select team.parent_team_id, responsible_team_line.depth + 1
        from responsible_team_line
        join public.team on team.id = responsible_team_line.team_id
        where team.parent_team_id is not null
          and responsible_team_line.depth < 64
      )
      select 1
      from responsible_team_line
      join public.member as reader on reader.team_id = responsible_team_line.team_id
      where reader.id = public.my_member()
    );
$$;

drop policy opportunity_readable_by_colleague on public.opportunity;
drop policy opportunity_written_by_colleague on public.opportunity;

create policy opportunity_readable_by_its_team on public.opportunity
  for select to authenticated
  using (
    company_id = internal.company_of_member(public.my_member())
    and internal.opportunity_visible_to_caller(owner_id, created_by)
  );

create policy opportunity_added_by_colleague on public.opportunity
  for insert to authenticated
  with check (company_id = internal.company_of_member(public.my_member()));

create policy opportunity_updated_by_its_team on public.opportunity
  for update to authenticated
  using (
    company_id = internal.company_of_member(public.my_member())
    and internal.opportunity_visible_to_caller(owner_id, created_by)
  )
  with check (company_id = internal.company_of_member(public.my_member()));

drop policy organization_written_by_colleague on public.organization;

create policy organization_added_by_colleague on public.organization
  for insert to authenticated
  with check (company_id = internal.company_of_member(public.my_member()));

create policy organization_updated_by_colleague on public.organization
  for update to authenticated
  using (company_id = internal.company_of_member(public.my_member()))
  with check (company_id = internal.company_of_member(public.my_member()));

drop policy contact_written_by_colleague on public.contact;

create policy contact_added_by_colleague on public.contact
  for insert to authenticated
  with check (company_id = internal.company_of_member(public.my_member()));

create policy contact_updated_by_colleague on public.contact
  for update to authenticated
  using (company_id = internal.company_of_member(public.my_member()))
  with check (company_id = internal.company_of_member(public.my_member()));

revoke delete on public.organization, public.contact, public.opportunity from anon, authenticated;

drop policy task_readable_by_colleague on public.task;

create policy task_readable_by_colleague on public.task
  for select to authenticated using (
    company_id = internal.company_of_member(public.my_member())
    and (
      opportunity_id is null
      or requester_id = public.my_member()
      or exists (
        select 1
        from public.task_participant
        where task_participant.task_id = task.id
          and task_participant.member_id = public.my_member()
      )
      or exists (
        select 1
        from public.opportunity
        where opportunity.id = task.opportunity_id
      )
    )
  );

alter function public.crm_opportunity_close(uuid, public.crm_stage, integer, timestamp with time zone, text, bigint, text)
  security invoker;

-- Postgres holds an updated row to the select policy as well, so an owner
-- handing a deal to another team could not write the change that takes it out
-- of their sight. Handing over is its own act, as Salesforce's transfer is:
-- whoever sees a deal may give it to anyone in the company and may lose sight
-- of it by doing so.
create function public.crm_opportunity_hand_over(
  target_opportunity_id uuid,
  target_owner_id uuid
)
returns public.opportunity
language plpgsql
security definer
set search_path = ''
as $$
declare
  actor_member uuid := public.my_member();
  actor_company uuid := internal.company_of_member(actor_member);
  saved public.opportunity;
begin
  select * into saved
  from public.opportunity
  where id = target_opportunity_id
    and company_id = actor_company
    and internal.opportunity_visible_to_caller(owner_id, created_by)
  for update;

  if saved.id is null then
    raise no_data_found using
      message = 'the opportunity is not one this member can see';
  end if;

  update public.opportunity
  set owner_id = target_owner_id,
      updated_by = actor_member
  where id = target_opportunity_id
  returning * into saved;

  return saved;
end;
$$;

revoke execute on function public.crm_opportunity_hand_over(uuid, uuid)
  from public, anon, service_role;
grant execute on function public.crm_opportunity_hand_over(uuid, uuid)
  to authenticated;

alter function public.enforce_crm_reference_consistency()
  security definer;
