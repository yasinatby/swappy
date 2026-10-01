-- Rounded distance between two postal-code centroids. Only this rounded
-- value is ever returned to clients (invariant 8). Null if either side has no
-- location.
create or replace function distance_km(lat1 double precision, lon1 double precision,
                                       lat2 double precision, lon2 double precision)
returns integer
language sql immutable as $$
    select case
        when lat1 is null or lat2 is null then null
        else greatest(1, round(
            2 * 6371 * asin(sqrt(
                power(sin(radians(lat2 - lat1) / 2), 2)
                + cos(radians(lat1)) * cos(radians(lat2)) * power(sin(radians(lon2 - lon1) / 2), 2)
            ))
        )::integer)
    end
$$;

alter table notifications add column if not exists read_at timestamptz;
create index if not exists notifications_user_idx on notifications (user_id, created_at desc);
