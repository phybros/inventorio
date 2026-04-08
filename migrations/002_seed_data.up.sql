-- Seed data: enum groups, categories, and attribute definitions
-- Uses hardcoded UUIDs so attribute_definitions can reference enum_group IDs

-- ============================================================
-- Enum Group UUIDs
-- ============================================================
-- package_types:         a0000000-0000-0000-0000-000000000001
-- logic_families:        a0000000-0000-0000-0000-000000000002
-- tolerance_bands:       a0000000-0000-0000-0000-000000000003
-- capacitor_types:       a0000000-0000-0000-0000-000000000004
-- dielectric_codes:      a0000000-0000-0000-0000-000000000005
-- gate_functions:        a0000000-0000-0000-0000-000000000006
-- diode_types:           a0000000-0000-0000-0000-000000000007
-- flipflop_types:        a0000000-0000-0000-0000-000000000008
-- edge_types:            a0000000-0000-0000-0000-000000000009
-- shift_reg_directions:  a0000000-0000-0000-0000-00000000000a
-- buffer_directions:     a0000000-0000-0000-0000-00000000000b
-- counter_types:         a0000000-0000-0000-0000-00000000000c
-- connector_types:       a0000000-0000-0000-0000-00000000000d
-- connector_gender:      a0000000-0000-0000-0000-00000000000e
-- mounting_types:        a0000000-0000-0000-0000-00000000000f
-- transistor_types:      a0000000-0000-0000-0000-000000000010
-- regulator_types:       a0000000-0000-0000-0000-000000000011

-- ============================================================
-- Category UUIDs
-- ============================================================
-- Resistors:             b0000000-0000-0000-0000-000000000001
-- Capacitors:            b0000000-0000-0000-0000-000000000002
-- Inductors:             b0000000-0000-0000-0000-000000000003
-- Diodes:                b0000000-0000-0000-0000-000000000004
-- Logic ICs:             b0000000-0000-0000-0000-000000000005
-- Gates:                 b0000000-0000-0000-0000-000000000006
-- Flip-Flops:            b0000000-0000-0000-0000-000000000007
-- Shift Registers:       b0000000-0000-0000-0000-000000000008
-- Buffers & Drivers:     b0000000-0000-0000-0000-000000000009
-- Counters:              b0000000-0000-0000-0000-00000000000a
-- Connectors:            b0000000-0000-0000-0000-00000000000b
-- Transistors:           b0000000-0000-0000-0000-00000000000c
-- Voltage Regulators:    b0000000-0000-0000-0000-00000000000d
-- Crystals & Oscillators:b0000000-0000-0000-0000-00000000000e
-- Misc / Unsorted:       b0000000-0000-0000-0000-00000000000f

-- ============================================================
-- ENUM GROUPS
-- ============================================================

INSERT INTO enum_groups (id, name) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'package_types'),
    ('a0000000-0000-0000-0000-000000000002', 'logic_families'),
    ('a0000000-0000-0000-0000-000000000003', 'tolerance_bands'),
    ('a0000000-0000-0000-0000-000000000004', 'capacitor_types'),
    ('a0000000-0000-0000-0000-000000000005', 'dielectric_codes'),
    ('a0000000-0000-0000-0000-000000000006', 'gate_functions'),
    ('a0000000-0000-0000-0000-000000000007', 'diode_types'),
    ('a0000000-0000-0000-0000-000000000008', 'flipflop_types'),
    ('a0000000-0000-0000-0000-000000000009', 'edge_types'),
    ('a0000000-0000-0000-0000-00000000000a', 'shift_register_directions'),
    ('a0000000-0000-0000-0000-00000000000b', 'buffer_directions'),
    ('a0000000-0000-0000-0000-00000000000c', 'counter_types'),
    ('a0000000-0000-0000-0000-00000000000d', 'connector_types'),
    ('a0000000-0000-0000-0000-00000000000e', 'connector_gender'),
    ('a0000000-0000-0000-0000-00000000000f', 'mounting_types'),
    ('a0000000-0000-0000-0000-000000000010', 'transistor_types'),
    ('a0000000-0000-0000-0000-000000000011', 'regulator_types');

-- ============================================================
-- ENUM VALUES
-- ============================================================

-- package_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000001', '0402',    '0402',    0),
    ('a0000000-0000-0000-0000-000000000001', '0603',    '0603',    1),
    ('a0000000-0000-0000-0000-000000000001', '0805',    '0805',    2),
    ('a0000000-0000-0000-0000-000000000001', '1206',    '1206',    3),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-8',   'DIP-8',   4),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-14',  'DIP-14',  5),
    ('a0000000-0000-0000-0000-000000000001', 'DIP-16',  'DIP-16',  6),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-8',  'SOIC-8',  7),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-14', 'SOIC-14', 8),
    ('a0000000-0000-0000-0000-000000000001', 'SOIC-16', 'SOIC-16', 9),
    ('a0000000-0000-0000-0000-000000000001', 'SSOP',    'SSOP',    10),
    ('a0000000-0000-0000-0000-000000000001', 'TSSOP',   'TSSOP',   11),
    ('a0000000-0000-0000-0000-000000000001', 'QFP',     'QFP',     12),
    ('a0000000-0000-0000-0000-000000000001', 'SOT-23',  'SOT-23',  13),
    ('a0000000-0000-0000-0000-000000000001', 'TO-92',   'TO-92',   14),
    ('a0000000-0000-0000-0000-000000000001', 'TO-220',  'TO-220',  15);

-- logic_families
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000002', 'HC',        'HC',        0),
    ('a0000000-0000-0000-0000-000000000002', 'HCT',       'HCT',       1),
    ('a0000000-0000-0000-0000-000000000002', 'LS',        'LS',        2),
    ('a0000000-0000-0000-0000-000000000002', 'ALS',       'ALS',       3),
    ('a0000000-0000-0000-0000-000000000002', 'AC',        'AC',        4),
    ('a0000000-0000-0000-0000-000000000002', 'ACT',       'ACT',       5),
    ('a0000000-0000-0000-0000-000000000002', 'LVC',       'LVC',       6),
    ('a0000000-0000-0000-0000-000000000002', 'LV',        'LV',        7),
    ('a0000000-0000-0000-0000-000000000002', 'F',         'F',         8),
    ('a0000000-0000-0000-0000-000000000002', 'S',         'S',         9),
    ('a0000000-0000-0000-0000-000000000002', 'CMOS_4000', 'CMOS 4000', 10);

-- tolerance_bands
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000003', '1pct',  '1%',  0),
    ('a0000000-0000-0000-0000-000000000003', '2pct',  '2%',  1),
    ('a0000000-0000-0000-0000-000000000003', '5pct',  '5%',  2),
    ('a0000000-0000-0000-0000-000000000003', '10pct', '10%', 3),
    ('a0000000-0000-0000-0000-000000000003', '20pct', '20%', 4);

-- capacitor_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000004', 'ceramic',      'Ceramic (MLCC)', 0),
    ('a0000000-0000-0000-0000-000000000004', 'electrolytic', 'Electrolytic',   1),
    ('a0000000-0000-0000-0000-000000000004', 'tantalum',     'Tantalum',       2),
    ('a0000000-0000-0000-0000-000000000004', 'film',         'Film',           3),
    ('a0000000-0000-0000-0000-000000000004', 'mica',         'Mica',           4);

-- dielectric_codes
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000005', 'C0G_NP0', 'C0G/NP0', 0),
    ('a0000000-0000-0000-0000-000000000005', 'X5R',     'X5R',     1),
    ('a0000000-0000-0000-0000-000000000005', 'X7R',     'X7R',     2),
    ('a0000000-0000-0000-0000-000000000005', 'Y5V',     'Y5V',     3),
    ('a0000000-0000-0000-0000-000000000005', 'Z5U',     'Z5U',     4);

-- gate_functions
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000006', 'AND',    'AND',    0),
    ('a0000000-0000-0000-0000-000000000006', 'NAND',   'NAND',   1),
    ('a0000000-0000-0000-0000-000000000006', 'OR',     'OR',     2),
    ('a0000000-0000-0000-0000-000000000006', 'NOR',    'NOR',    3),
    ('a0000000-0000-0000-0000-000000000006', 'XOR',    'XOR',    4),
    ('a0000000-0000-0000-0000-000000000006', 'XNOR',   'XNOR',  5),
    ('a0000000-0000-0000-0000-000000000006', 'NOT',    'NOT',    6),
    ('a0000000-0000-0000-0000-000000000006', 'Buffer', 'Buffer', 7);

-- diode_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000007', 'rectifier', 'Rectifier', 0),
    ('a0000000-0000-0000-0000-000000000007', 'schottky',  'Schottky',  1),
    ('a0000000-0000-0000-0000-000000000007', 'zener',     'Zener',     2),
    ('a0000000-0000-0000-0000-000000000007', 'led',       'LED',       3);

-- flipflop_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000008', 'D',  'D',  0),
    ('a0000000-0000-0000-0000-000000000008', 'JK', 'JK', 1),
    ('a0000000-0000-0000-0000-000000000008', 'SR', 'SR', 2),
    ('a0000000-0000-0000-0000-000000000008', 'T',  'T',  3);

-- edge_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000009', 'rising',  'Rising',  0),
    ('a0000000-0000-0000-0000-000000000009', 'falling', 'Falling', 1);

-- shift_register_directions
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000a', 'SIPO',          'Serial-In Parallel-Out', 0),
    ('a0000000-0000-0000-0000-00000000000a', 'PISO',          'Parallel-In Serial-Out', 1),
    ('a0000000-0000-0000-0000-00000000000a', 'bidirectional', 'Bidirectional',          2);

-- buffer_directions
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000b', 'uni', 'Unidirectional', 0),
    ('a0000000-0000-0000-0000-00000000000b', 'bi',  'Bidirectional',  1);

-- counter_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000c', 'binary',  'Binary',  0),
    ('a0000000-0000-0000-0000-00000000000c', 'decade',  'Decade',  1),
    ('a0000000-0000-0000-0000-00000000000c', 'up_down', 'Up/Down', 2);

-- connector_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000d', 'header', 'Header', 0),
    ('a0000000-0000-0000-0000-00000000000d', 'socket', 'Socket', 1),
    ('a0000000-0000-0000-0000-00000000000d', 'JST',    'JST',    2),
    ('a0000000-0000-0000-0000-00000000000d', 'barrel', 'Barrel', 3),
    ('a0000000-0000-0000-0000-00000000000d', 'USB',    'USB',    4);

-- connector_gender
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000e', 'male',   'Male',   0),
    ('a0000000-0000-0000-0000-00000000000e', 'female', 'Female', 1);

-- mounting_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-00000000000f', 'through_hole', 'Through-Hole', 0),
    ('a0000000-0000-0000-0000-00000000000f', 'SMD',          'SMD',          1);

-- transistor_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000010', 'NPN',      'NPN',      0),
    ('a0000000-0000-0000-0000-000000000010', 'PNP',      'PNP',      1),
    ('a0000000-0000-0000-0000-000000000010', 'N_MOSFET', 'N-MOSFET', 2),
    ('a0000000-0000-0000-0000-000000000010', 'P_MOSFET', 'P-MOSFET', 3),
    ('a0000000-0000-0000-0000-000000000010', 'JFET',     'JFET',     4);

-- regulator_types
INSERT INTO enum_values (enum_group_id, value, display_name, sort_order) VALUES
    ('a0000000-0000-0000-0000-000000000011', 'linear',    'Linear',    0),
    ('a0000000-0000-0000-0000-000000000011', 'switching', 'Switching', 1),
    ('a0000000-0000-0000-0000-000000000011', 'LDO',       'LDO',       2);

-- ============================================================
-- CATEGORIES
-- ============================================================

-- Top-level categories
INSERT INTO categories (id, name, description) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'Resistors',              'Fixed and variable resistors'),
    ('b0000000-0000-0000-0000-000000000002', 'Capacitors',             'All capacitor types'),
    ('b0000000-0000-0000-0000-000000000003', 'Inductors',              'Inductors and chokes'),
    ('b0000000-0000-0000-0000-000000000004', 'Diodes',                 'Rectifiers, Schottky, Zener, LEDs'),
    ('b0000000-0000-0000-0000-000000000005', 'Logic ICs',              'Digital logic integrated circuits'),
    ('b0000000-0000-0000-0000-00000000000b', 'Connectors',             'Headers, sockets, and connectors'),
    ('b0000000-0000-0000-0000-00000000000c', 'Transistors',            'BJTs, MOSFETs, JFETs'),
    ('b0000000-0000-0000-0000-00000000000d', 'Voltage Regulators',     'Linear and switching regulators'),
    ('b0000000-0000-0000-0000-00000000000e', 'Crystals & Oscillators', 'Crystals, oscillators, resonators'),
    ('b0000000-0000-0000-0000-00000000000f', 'Misc / Unsorted',        'Uncategorized components');

-- Logic IC child categories
INSERT INTO categories (id, name, parent_id, description) VALUES
    ('b0000000-0000-0000-0000-000000000006', 'Gates',              'b0000000-0000-0000-0000-000000000005', 'AND, OR, NAND, NOR, XOR gates'),
    ('b0000000-0000-0000-0000-000000000007', 'Flip-Flops',         'b0000000-0000-0000-0000-000000000005', 'D, JK, SR, T flip-flops'),
    ('b0000000-0000-0000-0000-000000000008', 'Shift Registers',    'b0000000-0000-0000-0000-000000000005', 'Serial/parallel shift registers'),
    ('b0000000-0000-0000-0000-000000000009', 'Buffers & Drivers',  'b0000000-0000-0000-0000-000000000005', 'Buffer and driver ICs'),
    ('b0000000-0000-0000-0000-00000000000a', 'Counters',           'b0000000-0000-0000-0000-000000000005', 'Binary, decade, up/down counters');

-- ============================================================
-- ATTRIBUTE DEFINITIONS
-- ============================================================

-- Resistors: resistance, tolerance, power_rating, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'resistance',   'Resistance',   'numeric', 'Ω', NULL, true,  0),
    ('b0000000-0000-0000-0000-000000000001', 'tolerance',    'Tolerance',    'enum',    NULL, 'a0000000-0000-0000-0000-000000000003', false, 10),
    ('b0000000-0000-0000-0000-000000000001', 'power_rating', 'Power Rating', 'numeric', 'W',  NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000001', 'package',      'Package',      'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 30);

-- Capacitors: capacitance, voltage_rating, capacitor_type, dielectric, tolerance, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000002', 'capacitance',     'Capacitance',     'numeric', 'F',  NULL, true,  0),
    ('b0000000-0000-0000-0000-000000000002', 'voltage_rating',  'Voltage Rating',  'numeric', 'V',  NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000002', 'capacitor_type',  'Capacitor Type',  'enum',    NULL, 'a0000000-0000-0000-0000-000000000004', false, 20),
    ('b0000000-0000-0000-0000-000000000002', 'dielectric',      'Dielectric',      'enum',    NULL, 'a0000000-0000-0000-0000-000000000005', false, 30),
    ('b0000000-0000-0000-0000-000000000002', 'tolerance',       'Tolerance',       'enum',    NULL, 'a0000000-0000-0000-0000-000000000003', false, 40),
    ('b0000000-0000-0000-0000-000000000002', 'package',         'Package',         'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 50);

-- Inductors: inductance, current_rating, dcr, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000003', 'inductance',     'Inductance',     'numeric', 'H',  NULL, true,  0),
    ('b0000000-0000-0000-0000-000000000003', 'current_rating', 'Current Rating', 'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000003', 'dcr',            'DCR',            'numeric', 'Ω', NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000003', 'package',        'Package',        'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 30);

-- Diodes: forward_voltage, max_current, reverse_voltage, type, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000004', 'forward_voltage',  'Forward Voltage',  'numeric', 'V',  NULL, false, 0),
    ('b0000000-0000-0000-0000-000000000004', 'max_current',      'Max Current',      'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000004', 'reverse_voltage',  'Reverse Voltage',  'numeric', 'V',  NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000004', 'diode_type',       'Type',             'enum',    NULL, 'a0000000-0000-0000-0000-000000000007', false, 30),
    ('b0000000-0000-0000-0000-000000000004', 'package',          'Package',          'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 40);

-- Logic ICs (parent): logic_family, supply_voltage_min, supply_voltage_max, propagation_delay, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000005', 'logic_family',       'Logic Family',       'enum',    NULL,  'a0000000-0000-0000-0000-000000000002', false, 0),
    ('b0000000-0000-0000-0000-000000000005', 'supply_voltage_min', 'Supply Voltage Min', 'numeric', 'V',   NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000005', 'supply_voltage_max', 'Supply Voltage Max', 'numeric', 'V',   NULL, false, 20),
    ('b0000000-0000-0000-0000-000000000005', 'propagation_delay',  'Propagation Delay',  'numeric', 'ns',  NULL, false, 30),
    ('b0000000-0000-0000-0000-000000000005', 'package',            'Package',            'enum',    NULL,  'a0000000-0000-0000-0000-000000000001', false, 40);

-- Gates: gate_function, num_gates
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000006', 'gate_function', 'Gate Function', 'enum',    NULL, 'a0000000-0000-0000-0000-000000000006', false, 0),
    ('b0000000-0000-0000-0000-000000000006', 'num_gates',     'Number of Gates', 'numeric', NULL, NULL, false, 10);

-- Flip-Flops: type, num_elements, edge
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000007', 'flipflop_type', 'Type',              'enum',    NULL, 'a0000000-0000-0000-0000-000000000008', false, 0),
    ('b0000000-0000-0000-0000-000000000007', 'num_elements',  'Number of Elements','numeric', NULL, NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000007', 'edge',          'Edge',              'enum',    NULL, 'a0000000-0000-0000-0000-000000000009', false, 20);

-- Shift Registers: num_bits, direction
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000008', 'num_bits',  'Number of Bits', 'numeric', NULL, NULL, false, 0),
    ('b0000000-0000-0000-0000-000000000008', 'direction', 'Direction',      'enum',    NULL, 'a0000000-0000-0000-0000-00000000000a', false, 10);

-- Buffers & Drivers: num_channels, tri_state, direction
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-000000000009', 'num_channels', 'Number of Channels', 'numeric', NULL, NULL, false, 0),
    ('b0000000-0000-0000-0000-000000000009', 'tri_state',    'Tri-State',          'boolean', NULL, NULL, false, 10),
    ('b0000000-0000-0000-0000-000000000009', 'direction',    'Direction',          'enum',    NULL, 'a0000000-0000-0000-0000-00000000000b', false, 20);

-- Counters: num_bits, type
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000a', 'num_bits',     'Number of Bits', 'numeric', NULL, NULL, false, 0),
    ('b0000000-0000-0000-0000-00000000000a', 'counter_type', 'Type',           'enum',    NULL, 'a0000000-0000-0000-0000-00000000000c', false, 10);

-- Connectors: num_pins, pitch, type, gender, mounting
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000b', 'num_pins',       'Number of Pins', 'numeric', NULL,  NULL, false, 0),
    ('b0000000-0000-0000-0000-00000000000b', 'pitch',          'Pitch',          'numeric', 'mm',  NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000b', 'connector_type', 'Type',           'enum',    NULL,  'a0000000-0000-0000-0000-00000000000d', false, 20),
    ('b0000000-0000-0000-0000-00000000000b', 'gender',         'Gender',         'enum',    NULL,  'a0000000-0000-0000-0000-00000000000e', false, 30),
    ('b0000000-0000-0000-0000-00000000000b', 'mounting',       'Mounting',       'enum',    NULL,  'a0000000-0000-0000-0000-00000000000f', false, 40);

-- Transistors: type, max_current, max_voltage, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000c', 'transistor_type', 'Type',        'enum',    NULL, 'a0000000-0000-0000-0000-000000000010', false, 0),
    ('b0000000-0000-0000-0000-00000000000c', 'max_current',     'Max Current', 'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000c', 'max_voltage',     'Max Voltage', 'numeric', 'V',  NULL, false, 20),
    ('b0000000-0000-0000-0000-00000000000c', 'package',         'Package',     'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 30);

-- Voltage Regulators: output_voltage, output_current, type, dropout_voltage, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000d', 'output_voltage',  'Output Voltage',  'numeric', 'V',  NULL, false, 0),
    ('b0000000-0000-0000-0000-00000000000d', 'output_current',  'Output Current',  'numeric', 'A',  NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000d', 'regulator_type',  'Type',            'enum',    NULL, 'a0000000-0000-0000-0000-000000000011', false, 20),
    ('b0000000-0000-0000-0000-00000000000d', 'dropout_voltage', 'Dropout Voltage', 'numeric', 'V',  NULL, false, 30),
    ('b0000000-0000-0000-0000-00000000000d', 'package',         'Package',         'enum',    NULL, 'a0000000-0000-0000-0000-000000000001', false, 40);

-- Crystals & Oscillators: frequency, load_capacitance, tolerance, package
INSERT INTO attribute_definitions (category_id, name, display_name, data_type, unit, enum_group_id, is_required, sort_order) VALUES
    ('b0000000-0000-0000-0000-00000000000e', 'frequency',        'Frequency',        'numeric', 'Hz',  NULL, true,  0),
    ('b0000000-0000-0000-0000-00000000000e', 'load_capacitance', 'Load Capacitance', 'numeric', 'F',   NULL, false, 10),
    ('b0000000-0000-0000-0000-00000000000e', 'tolerance',        'Tolerance',        'numeric', 'ppm', NULL, false, 20),
    ('b0000000-0000-0000-0000-00000000000e', 'package',          'Package',          'enum',    NULL,  'a0000000-0000-0000-0000-000000000001', false, 30);
