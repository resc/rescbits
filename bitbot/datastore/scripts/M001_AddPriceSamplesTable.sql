CREATE TABLE public.pricesamples (
  -- The id is generated from the file name
  Id        INT PRIMARY KEY,
  AppliedOn TIMESTAMP    NOT NULL DEFAULT NOW(),
  Type      CHAR(1)      NOT NULL,
  Name      VARCHAR(255) NOT NULL,
  Script    VARCHAR      NOT NULL
)