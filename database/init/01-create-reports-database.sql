-- Also run by postgres-init for existing data volumes; CREATE DATABASE cannot run in a transaction.
SELECT format('CREATE DATABASE %I OWNER %I', 'reports', current_user)
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'reports')
\gexec
