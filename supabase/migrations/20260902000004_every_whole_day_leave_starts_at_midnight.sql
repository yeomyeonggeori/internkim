update public.leave
set starts_at = whole.day_start,
	ends_at = whole.day_start + make_interval(days => whole.covered_days)
from (
	select held.id,
		(((held.starts_at at time zone company.timezone)::date)::timestamp at time zone company.timezone) as day_start,
		greatest(
			1,
			round(extract(epoch from (held.ends_at - held.starts_at)) / 86400)::integer
		) as covered_days
	from public.leave as held
	join public.member on member.id = held.member_id
	join public.company on company.id = member.company_id
	where held.days > 0.5
) as whole
where public.leave.id = whole.id
	and (
		public.leave.starts_at is distinct from whole.day_start
		or public.leave.ends_at is distinct from whole.day_start + make_interval(days => whole.covered_days)
	);
