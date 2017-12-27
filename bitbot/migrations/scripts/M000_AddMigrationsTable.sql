CREATE TABLE public.migrations (
  Id        INT PRIMARY KEY,
  AppliedOn TIMESTAMP    NOT NULL DEFAULT NOW(),
  Name      VARCHAR(255) NOT NULL
)