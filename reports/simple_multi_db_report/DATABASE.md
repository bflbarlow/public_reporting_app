# ClassicModels Database Documentation

## Overview
The ClassicModels database is a sample database for a classic car and scale model retailer. It is a widely-used sample database in the MySQL community that models a typical business with customers, orders, products, employees, and payments. This documentation provides report developers with schema details and sample data to understand the database structure for writing reports.

## Connection Details
- **Database Name**: classicmodels
- **Host**: localhost
- **Username**: root
- **Password**: (none - empty password) *Note: This is for local development only. In production, use a secure password.*
- **Port**: 3306 (default MySQL port)

## Database Tables (8 total tables)
1. `customers` - Customer information
2. `employees` - Employee records
3. `offices` - Office locations
4. `orders` - Customer orders
5. `orderdetails` - Line items for each order
6. `payments` - Customer payment records
7. `products` - Product catalog
8. `productlines` - Product categories/descriptions

## Table Row Counts
| Table | Row Count |
|-------|-----------|
| customers | 122 |
| employees | 23 |
| offices | 7 |
| orders | 326 |
| orderdetails | 2996 |
| payments | 273 |
| products | 110 |
| productlines | 7 |

## Table Schema Details

### customers Table
Primary table storing customer information.

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| customerNumber | int | NO | PRI | Unique customer identifier |
| customerName | varchar(50) | NO |  | Company/individual name |
| contactLastName | varchar(50) | NO |  | Last name of contact person |
| contactFirstName | varchar(50) | NO |  | First name of contact person |
| phone | varchar(50) | NO |  | Phone number |
| addressLine1 | varchar(50) | NO |  | Primary address line |
| addressLine2 | varchar(50) | YES |  | Secondary address line (optional) |
| city | varchar(50) | NO |  | City |
| state | varchar(50) | YES |  | State/province (optional) |
| postalCode | varchar(15) | YES |  | Postal/ZIP code |
| country | varchar(50) | NO |  | Country |
| salesRepEmployeeNumber | int | YES | MUL | Employee who handles this customer (foreign key to employees) |
| creditLimit | decimal(10,2) | YES |  | Maximum credit allowed |

**Sample Data (first 3 rows):**
```
customerNumber,customerName,contactLastName,contactFirstName,phone,addressLine1,addressLine2,city,state,postalCode,country,salesRepEmployeeNumber,creditLimit
103,Atelier graphique,Schmitt,Carine ,40.32.2555,54, rue Royale,NULL,Nantes,NULL,44000,France,1370,21000.00
112,Signal Gift Stores,King,Jean,7025551838,8489 Strong St.,NULL,Las Vegas,NV,83030,USA,1166,71800.00
114,Australian Collectors, Co.,Ferguson,Peter,03 9520 4555,636 St Kilda Road,Level 3,Melbourne,Victoria,3004,Australia,1611,117300.00
```

**Common Report Use Cases:**
- Customer contact lists
- Sales territory assignments
- Credit limit reports
- Geographic customer distribution

### employees Table
Employee information and organization hierarchy.

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| employeeNumber | int | NO | PRI | Unique employee identifier |
| lastName | varchar(50) | NO |  | Last name |
| firstName | varchar(50) | NO |  | First name |
| extension | varchar(10) | NO |  | Phone extension |
| email | varchar(100) | NO |  | Email address |
| officeCode | varchar(10) | NO | MUL | Office location (foreign key to offices) |
| reportsTo | int | YES | MUL | Manager's employee number (self-referencing foreign key) |
| jobTitle | varchar(50) | NO |  | Job title/position |

**Sample Data (first 3 rows):**
```
employeeNumber,lastName,firstName,extension,email,officeCode,reportsTo,jobTitle
1002,Murphy,Diane,x5800,dmurphy@classicmodelcars.com,1,NULL,President
1056,Patterson,Mary,x4611,mpatterso@classicmodelcars.com,1,1002,VP Sales
1076,Firrelli,Jeff,x9273,jfirrelli@classicmodelcars.com,1,1002,VP Marketing
```

**Common Report Use Cases:**
- Employee directory
- Organizational charts
- Sales team assignments
- Office staffing reports

### offices Table
Office locations where employees work.

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| officeCode | varchar(10) | NO | PRI | Unique office identifier |
| city | varchar(50) | NO |  | City name |
| phone | varchar(50) | NO |  | Office phone number |
| addressLine1 | varchar(50) | NO |  | Primary address |
| addressLine2 | varchar(50) | YES |  | Secondary address (optional) |
| state | varchar(50) | YES |  | State/province (optional) |
| country | varchar(50) | NO |  | Country |
| postalCode | varchar(15) | NO |  | Postal/ZIP code |
| territory | varchar(10) | NO |  | Sales territory |

**Sample Data (first 3 rows):**
```
officeCode,city,phone,addressLine1,addressLine2,state,country,postalCode,territory
1,San Francisco,+1 650 219 4782,100 Market Street,Suite 300,CA,USA,94080,NA
2,Boston,+1 215 837 0825,1550 Court Place,Suite 102,MA,USA,02107,NA
3,NYC,+1 212 555 3000,523 East 53rd Street,apt. 5A,NY,USA,10022,NA
```

**Common Report Use Cases:**
- Office location directory
- Geographic sales territories
- Employee distribution by office

### orders Table
Customer order headers (main order information).

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| orderNumber | int | NO | PRI | Unique order identifier |
| orderDate | date | NO |  | Date order was placed |
| requiredDate | date | NO |  | Date order is required by customer |
| shippedDate | date | YES |  | Date order was shipped (NULL if not yet shipped) |
| status | varchar(15) | NO |  | Order status (e.g., 'Shipped', 'Cancelled', 'On Hold') |
| comments | text | YES |  | Order comments/notes |
| customerNumber | int | NO | MUL | Customer who placed the order (foreign key to customers) |

**Sample Data (first 3 rows):**
```
orderNumber,orderDate,requiredDate,shippedDate,status,comments,customerNumber
10100,2003-01-06,2003-01-13,2003-01-10,Shipped,NULL,363
10101,2003-01-09,2003-01-18,2003-01-11,Shipped,Check on availability.,128
10102,2003-01-10,2003-01-18,2003-01-14,Shipped,NULL,181
```

**Common Report Use Cases:**
- Order status reports
- Sales by date/month/quarter
- Order fulfillment timelines
- Customer order history

### orderdetails Table
Line items for each order (order details).

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| orderNumber | int | NO | PRI | Order identifier (foreign key to orders) |
| productCode | varchar(15) | NO | PRI | Product identifier (foreign key to products) |
| quantityOrdered | int | NO |  | Quantity ordered |
| priceEach | decimal(10,2) | NO |  | Price per unit at time of order |
| orderLineNumber | smallint | NO |  | Line number on the order |

**Sample Data (first 3 rows):**
```
orderNumber,productCode,quantityOrdered,priceEach,orderLineNumber
10100,S18_1749,30,136.00,3
10100,S18_2248,50,55.09,2
10100,S18_4409,22,75.46,4
```

**Common Report Use Cases:**
- Order line item details
- Product sales analysis
- Revenue by product
- Quantity ordered reports

### payments Table
Customer payment records.

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| customerNumber | int | NO | PRI | Customer identifier (foreign key to customers) |
| checkNumber | varchar(50) | NO | PRI | Check/payment reference number |
| paymentDate | date | NO |  | Date of payment |
| amount | decimal(10,2) | NO |  | Payment amount |

**Sample Data (first 3 rows):**
```
customerNumber,checkNumber,paymentDate,amount
103,HQ336336,2004-10-19,6066.78
103,JM555205,2003-06-05,14571.44
103,OM314933,2004-12-18,1676.14
```

**Common Report Use Cases:**
- Payment history reports
- Accounts receivable
- Customer payment patterns
- Revenue collection reports

### products Table
Product catalog/inventory.

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| productCode | varchar(15) | NO | PRI | Unique product identifier |
| productName | varchar(70) | NO |  | Product name |
| productLine | varchar(50) | NO | MUL | Product category (foreign key to productlines) |
| productScale | varchar(10) | NO |  | Scale (e.g., '1:10', '1:24') |
| productVendor | varchar(50) | NO |  | Vendor/manufacturer |
| productDescription | text | NO |  | Detailed product description |
| quantityInStock | smallint | NO |  | Current inventory quantity |
| buyPrice | decimal(10,2) | NO |  | Cost price |
| MSRP | decimal(10,2) | NO |  | Manufacturer's suggested retail price |

**Sample Data (first 3 rows):**
```
productCode,productName,productLine,productScale,productVendor,productDescription,quantityInStock,buyPrice,MSRP
S10_1678,1969 Harley Davidson Ultimate Chopper,Motorcycles,1:10,Min Lin Diecast,This replica features working kickstand, front suspension, gear-shift lever, footbrake lever, drive chain, wheels and steering. All parts are particularly delicate due to their precise scale and require special care and attention.,7933,48.81,95.70
S10_1949,1952 Alpine Renault 1300,Classic Cars,1:10,Classic Metal Creations,Turnable front wheels; steering function; detailed interior; detailed engine; opening hood; opening trunk; opening doors; and detailed chassis.,7305,98.58,214.30
S10_2016,1996 Moto Guzzi 1100i,Motorcycles,1:10,Highway 66 Mini Classics,Official Moto Guzzi logos and insignias, saddle bags located on side of motorcycle, detailed engine, working steering, working suspension, two leather seats, luggage rack, dual exhaust pipes, small saddle bag located on handle bars, two-tone paint with chrome accents, superior die-cast detail , rotating wheels , working kick stand, diecast metal with plastic parts and baked enamel finish.,6625,68.99,118.94
```

**Common Report Use Cases:**
- Product catalog
- Inventory management
- Product pricing analysis
- Vendor performance

### productlines Table
Product categories with descriptive information.

**Columns:**
| Column Name | Data Type | Nullable | Key | Description |
|-------------|-----------|----------|-----|-------------|
| productLine | varchar(50) | NO | PRI | Product category identifier |
| textDescription | varchar(4000) | YES |  | Text description of product line |
| htmlDescription | mediumtext | YES |  | HTML description (typically NULL) |
| image | mediumblob | YES |  | Product line image (typically NULL) |

**Sample Data (first 3 rows):**
```
productLine,textDescription,htmlDescription,image
Classic Cars,Attention car enthusiasts: Make your wildest car ownership dreams come true. Whether you are looking for classic muscle cars, dream sports cars or movie-inspired miniatures, you will find great choices in this category. These replicas feature superb attention to detail and craftsmanship and offer features such as working steering system, opening forward compartment, opening rear trunk with removable spare wheel, 4-wheel independent spring suspension, and so on. The models range in size from 1:10 to 1:24 scale and include numerous limited edition and several out-of-production vehicles. All models include a certificate of authenticity from their manufacturers and come fully assembled and ready for display in the home or office.,NULL,NULL
Motorcycles,Our motorcycles are state of the art replicas of classic as well as contemporary motorcycle legends such as Harley Davidson, Ducati and Vespa. Models contain stunning details such as official logos, rotating wheels, working kickstand, front suspension, gear-shift lever, footbrake lever, and drive chain. Materials used include diecast and plastic. The models range in size from 1:10 to 1:50 scale and include numerous limited edition and several out-of-production vehicles. All models come fully assembled and ready for display in the home or office. Most include a certificate of authenticity.,NULL,NULL
Planes,Unique, diecast airplane and helicopter replicas suitable for collections, as well as home, office or classroom decorations. Models contain stunning details such as official logos and insignias, rotating jet engines and propellers, retractable wheels, and so on. Most come fully assembled and with a certificate of authenticity from their manufacturers.,NULL,NULL
```

**Common Report Use Cases:**
- Product category listings
- Marketing content for product lines
- Product line descriptions for catalogs

## Key Relationships

### Foreign Key Relationships
1. `customers.salesRepEmployeeNumber` → `employees.employeeNumber`
2. `employees.officeCode` → `offices.officeCode`
3. `employees.reportsTo` → `employees.employeeNumber` (self-reference)
4. `orders.customerNumber` → `customers.customerNumber`
5. `orderdetails.orderNumber` → `orders.orderNumber`
6. `orderdetails.productCode` → `products.productCode`
7. `payments.customerNumber` → `customers.customerNumber`
8. `products.productLine` → `productlines.productLine`

## Common Report Queries

### Sales Report (Orders with Customer and Employee)
```sql
SELECT 
    o.orderNumber, 
    o.orderDate, 
    o.status,
    c.customerName,
    CONCAT(e.firstName, ' ', e.lastName) AS salesRep
FROM orders o
JOIN customers c ON o.customerNumber = c.customerNumber
JOIN employees e ON c.salesRepEmployeeNumber = e.employeeNumber
WHERE o.orderDate BETWEEN '2003-01-01' AND '2003-12-31';
```

### Product Sales Summary
```sql
SELECT 
    p.productCode,
    p.productName,
    p.productLine,
    SUM(od.quantityOrdered) AS totalQuantity,
    SUM(od.quantityOrdered * od.priceEach) AS totalRevenue
FROM orderdetails od
JOIN products p ON od.productCode = p.productCode
GROUP BY p.productCode, p.productName, p.productLine
ORDER BY totalRevenue DESC;
```

### Customer Payment Summary
```sql
SELECT 
    c.customerNumber,
    c.customerName,
    c.country,
    COUNT(p.checkNumber) AS paymentCount,
    SUM(p.amount) AS totalPaid
FROM customers c
LEFT JOIN payments p ON c.customerNumber = p.customerNumber
GROUP BY c.customerNumber, c.customerName, c.country
ORDER BY totalPaid DESC;
```

## Notes for Report Developers
- The database contains historical data from approximately 2003-2005
- Order status values include: 'Shipped', 'Cancelled', 'On Hold', 'Disputed', 'Resolved'
- Product scales range from 1:10 to 1:50
- Some fields like `htmlDescription` and `image` in productlines are typically NULL
- The `comments` field in orders may contain useful order-specific notes

## Getting Help
For additional database exploration, connect using:
```bash
mysql -u root classicmodels
```
Then explore with:
- `SHOW TABLES;`
- `DESCRIBE <table_name>;`
- `SELECT COUNT(*) FROM <table_name>;`