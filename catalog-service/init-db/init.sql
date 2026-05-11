-- Se prepara el entorno para el uso de UUIDs
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Se crean las tablas comercios y productos
CREATE TABLE IF NOT EXISTS comercios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre VARCHAR(255) NOT NULL,
    direccion TEXT NOT NULL,
    activo BOOLEAN DEFAULT true
);

CREATE TABLE IF NOT EXISTS productos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    comercio_id UUID REFERENCES comercios(id) ON DELETE CASCADE,
    nombre VARCHAR(255) NOT NULL,
    precio DECIMAL(10, 2) NOT NULL,
    disponible BOOLEAN DEFAULT true
);

-- Se insertan datos semilla con IDs fijos (SOLO caracteres 0-9 y a-f)
INSERT INTO comercios (id, nombre, direccion, activo)
VALUES (
    'c1111111-1111-1111-1111-111111111111', 
    'Comercio Demo (Burger Queen)', 
    'Av. Siempre Viva 742', 
    true
)
ON CONFLICT (id) DO UPDATE 
SET 
    nombre = EXCLUDED.nombre, 
    direccion = EXCLUDED.direccion, 
    activo = EXCLUDED.activo;

INSERT INTO comercios (id, nombre, direccion, activo)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'Tienda Central', 'Calle 1 #100', true),
    ('00000000-0000-0000-0000-000000000002', 'Pizzeria Norte', 'Avenida 2 #200', true)
ON CONFLICT (id) DO UPDATE
SET
    nombre = EXCLUDED.nombre,
    direccion = EXCLUDED.direccion,
    activo = EXCLUDED.activo;

-- Se insertan los productos (He cambiado la 'p' inicial por un '1')
INSERT INTO productos (id, comercio_id, nombre, precio, disponible)
VALUES 
(
    '11111111-1111-1111-1111-111111111111', 
    'c1111111-1111-1111-1111-111111111111', 
    'Hamburguesa Demo', 
    10.00, 
    true
),
(
    '22222222-2222-2222-2222-222222222222', 
    'c1111111-1111-1111-1111-111111111111', 
    'Bebida Demo', 
    2.50, 
    true
)
ON CONFLICT (id) DO UPDATE 
SET 
    nombre = EXCLUDED.nombre, 
    precio = EXCLUDED.precio, 
    disponible = EXCLUDED.disponible;

INSERT INTO productos (id, comercio_id, nombre, precio, disponible)
VALUES
    ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'Combo Clasico', 12.00, true),
    ('00000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000002', 'Pizza Margarita', 15.00, true)
ON CONFLICT (id) DO UPDATE
SET
    nombre = EXCLUDED.nombre,
    precio = EXCLUDED.precio,
    disponible = EXCLUDED.disponible;
