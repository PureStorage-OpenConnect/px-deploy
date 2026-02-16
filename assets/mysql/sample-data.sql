USE pxdemo;
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    first_name VARCHAR(50),
    last_name VARCHAR(50)
);
INSERT INTO users (first_name, last_name) VALUES 
('Alice', 'Anderson'),
('Bob', 'Barker'),
('Charlie', 'Chaplin');