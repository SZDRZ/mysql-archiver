

-- 商品表
CREATE TABLE products (
    product_id INT AUTO_INCREMENT PRIMARY KEY,
    product_name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 商品包装表
CREATE TABLE product_packages (
    package_id INT AUTO_INCREMENT PRIMARY KEY,
    product_id INT NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    package_type VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES products(product_id) ON DELETE CASCADE
);

-- 商品包装条码表
CREATE TABLE package_barcodes (
    barcode_id INT AUTO_INCREMENT PRIMARY KEY,
    package_id INT NOT NULL,
    barcode VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (package_id) REFERENCES product_packages(package_id) ON DELETE CASCADE
);

-- 插入商品数据
INSERT INTO products (product_name, description) VALUES
('商品A', '这是商品A的描述'),
('商品B', '这是商品B的描述'),
('商品C', '这是商品C的描述');

-- 插入商品包装数据
INSERT INTO product_packages (product_id, package_name, package_type) VALUES
(1, '商品A-包装1', '盒装'),
(1, '商品A-包装2', '袋装'),
(2, '商品B-包装1', '瓶装'),
(2, '商品B-包装2', '罐装'),
(3, '商品C-包装1', '箱装'),
(3, '商品C-包装2', '桶装');

-- 插入商品包装条码数据
INSERT INTO package_barcodes (package_id, barcode) VALUES
(1, '1234567890123'),
(1, '1234567890124'),
(2, '123456789B125'),
(2, '123456789R125'),
(2, '123456789V125'),
(3, '1234567890126'),
(4, '1234567890127'),
(5, '1234567890128'),
(6, '1234567890129');
