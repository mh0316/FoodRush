CREATE TABLE IF NOT EXISTS payments (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL UNIQUE,
  user_id TEXT NOT NULL,
  amount BIGINT NOT NULL,
  metodo_pago_token TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS outbox (
  id UUID PRIMARY KEY,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  headers JSONB,
  processed BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON outbox(created_at) WHERE processed = FALSE;

INSERT INTO payments (id, order_id, user_id, amount, metodo_pago_token, status)
VALUES
  ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 20, 'tok_ana', 'APPROVED')
ON CONFLICT (order_id) DO UPDATE
SET
  user_id = EXCLUDED.user_id,
  amount = EXCLUDED.amount,
  metodo_pago_token = EXCLUDED.metodo_pago_token,
  status = EXCLUDED.status,
  created_at = NOW();
