INSERT INTO products (sku, name, price, stock) VALUES
  ('SKU-001', 'Baut Hex M8x40 (isi 50)', 12500, 100),
  ('SKU-002', 'Mur Hex M8 (isi 100)', 9000, 50),
  ('SKU-003', 'Kunci Pas Set 8 pcs', 185000, 0),
  ('SKU-004', 'Bearing 6204-2RS', 38000, 25),
  ('SKU-005', 'Selang Hidrolik 1/2 inch (per meter)', 65000, 8),
  ('SKU-006', 'Oli Hidrolik ISO VG 46 (20L)', 720000, 40),
  ('SKU-007', 'Kabel NYY 4x10mm (per meter)', 45000, 500),
  ('SKU-008', 'Sarung Tangan Las Kulit', 35000, 15),
  ('SKU-009', 'Helm Safety Proyek', 45000, 10),
  ('SKU-010', 'Pallet Besi Heavy Duty', 650000, 0),
  ('SKU-011', 'Mata Gerinda Potong 4 inch', 11000, 220),
  ('SKU-012', 'Gerinda Tangan 4 inch 710W', 450000, 60),
  ('SKU-013', 'Hand Pallet Manual 2 Ton', 4200000, 4),
  ('SKU-014', 'Sepatu Safety Steel Toe', 285000, 120),
  ('SKU-015', 'Kompresor Udara 1 HP', 2150000, 35);

  INSERT INTO stock_movements (product_id, type, qty, note)
  SELECT id, 'in', stock, 'Initial stock' FROM products WHERE stock > 0;

  INSERT INTO sales_orders (customer, total) VALUES ('Budi', 143000);

  INSERT INTO sales_order_items (order_id, sku, qty, unit_price) VALUES
  (1, 'SKU-001', 10, 12500),
  (1, 'SKU-002', 2, 9000);

  INSERT INTO stock_movements (product_id, type, qty, note) VALUES
  (1, 'out', 10, 'Sales order #1'),
  (2, 'out', 2, 'Sales order #1');

  UPDATE products SET stock = stock - 10 WHERE sku = 'SKU-001';
  UPDATE products SET stock = stock - 2 WHERE sku = 'SKU-002';

  UPDATE products SET is_active = FALSE WHERE sku = 'SKU-015';
