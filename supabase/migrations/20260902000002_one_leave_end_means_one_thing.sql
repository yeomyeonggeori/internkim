update public.leave
set ends_at = (((public.leave.ends_at at time zone company.timezone)::date + 1)::timestamp at time zone company.timezone)
from public.member
join public.company on company.id = member.company_id
where member.id = public.leave.member_id
	and public.leave.days > 0.5
	and (public.leave.ends_at at time zone company.timezone)::time = '23:59:59.999';
