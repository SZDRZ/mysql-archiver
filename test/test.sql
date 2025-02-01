CREATE TABLE Orders (
    Id INT PRIMARY KEY AUTO_INCREMENT,
    OrderNumber VARCHAR(50) NOT NULL,
    CustomerName VARCHAR(100) NOT NULL,
    OrderDate DATETIME NOT NULL,
    TotalAmount DECIMAL(10, 2) NOT NULL,
    Status VARCHAR(20) NOT NULL,
    CreatedTime DATETIME NOT NULL,
    IsDeleted TINYINT NOT NULL DEFAULT 0
);

CREATE TABLE OrderDetails (
    DetailId INT PRIMARY KEY AUTO_INCREMENT,
    OrderId INT NOT NULL,
    ProductId INT NOT NULL,
    ProductName VARCHAR(100) NOT NULL,
    Quantity INT NOT NULL,
    UnitPrice DECIMAL(10, 2) NOT NULL,
    FOREIGN KEY (OrderId) REFERENCES Orders(Id)
);

INSERT INTO Orders (OrderNumber, CustomerName, OrderDate, TotalAmount, Status, CreatedTime, IsDeleted)
VALUES
('ORD001', 'Alice', '2025-01-01 10:00:00', 250.00, 'Pending', '2025-01-01 10:00:00', 0),
('ORD002', 'Bob', '2025-01-02 11:00:00', 150.00, 'Shipped', '2025-01-02 11:00:00', 0),
('ORD003', 'Charlie', '2025-01-03 12:00:00', 300.00, 'Pending', '2025-01-03 12:00:00', 0),
('ORD004', 'David', '2025-01-04 13:00:00', 200.00, 'Completed', '2025-01-04 13:00:00', 0),
('ORD005', 'Eve', '2025-01-05 14:00:00', 100.00, 'Pending', '2025-01-05 14:00:00', 0),
('ORD006', 'Frank', '2025-01-06 15:00:00', 400.00, 'Shipped', '2025-01-06 15:00:00', 0),
('ORD007', 'Grace', '2025-01-07 16:00:00', 250.00, 'Completed', '2025-01-07 16:00:00', 0),
('ORD008', 'Hannah', '2025-01-08 17:00:00', 300.00, 'Pending', '2025-01-08 17:00:00', 0),
('ORD009', 'Ian', '2025-01-09 18:00:00', 150.00, 'Shipped', '2025-01-09 18:00:00', 0),
('ORD010', 'Julia', '2025-01-10 19:00:00', 200.00, 'Completed', '2025-01-10 19:00:00', 0);


-- OrderId = 1 的明细
INSERT INTO OrderDetails (OrderId, ProductId, ProductName, Quantity, UnitPrice)
VALUES
(1, 101, 'Product A', 2, 50.00),
(1, 102, 'Product B', 1, 100.00),
(1, 103, 'Product C', 3, 30.00),
(1, 104, 'Product D', 1, 20.00),
(1, 105, 'Product E', 2, 50.00);

-- OrderId = 2 的明细
INSERT INTO OrderDetails (OrderId, ProductId, ProductName, Quantity, UnitPrice)
VALUES
(2, 106, 'Product F', 1, 150.00),
(2, 107, 'Product G', 2, 50.00),
(2, 108, 'Product H', 1, 20.00),
(2, 109, 'Product I', 3, 10.00),
(2, 110, 'Product J', 1, 20.00);

-- OrderId = 3 的明细
INSERT INTO OrderDetails (OrderId, ProductId, ProductName, Quantity, UnitPrice)
VALUES
(3, 111, 'Product K', 2, 100.00),
(3, 112, 'Product L', 1, 50.00),
(3, 113, 'Product M', 3, 50.00),
(3, 114, 'Product N', 1, 100.00),
(3, 115, 'Product O', 2, 50.00);

-- OrderId = 4 的明细
INSERT INTO OrderDetails (OrderId, ProductId, ProductName, Quantity, UnitPrice)
VALUES
(4, 116, 'Product P', 1, 50.00),
(4, 117, 'Product Q', 2, 50.00),
(4, 118, 'Product R', 1, 50.00),
(4, 119, 'Product S', 3, 50.00),
(4, 120, 'Product T', 1, 50.00);

-- OrderId = 5 的明细
INSERT INTO OrderDetails (OrderId, ProductId, ProductName, Quantity, UnitPrice)
VALUES
(5, 121, 'Product U', 2, 50.00),
(5, 122, 'Product V', 1, 50.00),
(5, 123, 'Product W', 3, 50.00),
(5, 124, 'Product X', 1, 50.00),
(5, 125, 'Product Y', 2, 50.00);

-- OrderId = 6 的明细
INSERT INTO OrderDetails (OrderId, ProductId, ProductName, Quantity, UnitPrice)
VALUES
(6, 126, 'Product Z', 1, 50.00),
(6, 127, 'Product AA', 2, 50.00),
(6, 128, 'Product AB', 1, 50.00),
(6, 129, 'Product AC', 3, 50.00),
(6, 130, 'Product AD', 1, 50.00);

-- OrderId = 7 的明细
INSERT INTO OrderDetails (OrderId, ProductId, ProductName, Quantity, UnitPrice)
VALUES
(7, 131, 'Product AE', 2, 50.00),
(7, 132, 'Product AF', 1, 50.00),
(7, 133, 'Product AG', 3, 50.00),
(7, 134, 'Product AH', 1, 50.00),
(7, 135, 'Product AI', 2, 50.00);
