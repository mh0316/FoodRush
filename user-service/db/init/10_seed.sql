INSERT INTO users (id, nombre, correo, payment_token, password)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  'Usuario Demo',
  'demo@foodrush.local',
  'tok_demo_foodrush',
  'demo_password_hash'
)
ON CONFLICT (correo) DO UPDATE
SET
  nombre = EXCLUDED.nombre,
  payment_token = EXCLUDED.payment_token,
  password = EXCLUDED.password,
  updated_at = NOW();
