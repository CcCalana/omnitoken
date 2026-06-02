\set ON_ERROR_STOP on

SELECT (length(:'password') > 0)::text AS admin_password_present \gset
\if :admin_password_present
\else
  \echo 'ADMIN_INITIAL_PASSWORD is required'
  \quit 1
\endif

WITH updated AS (
  UPDATE users
  SET password_hash = crypt(:'password', gen_salt('bf')),
      updated_at = now()
  WHERE email = 'admin@democorp.local'
  RETURNING id
)
SELECT (COUNT(*) = 1)::text AS admin_password_updated
FROM updated \gset

\if :admin_password_updated
\else
  \echo 'admin@democorp.local was not found'
  \quit 1
\endif
