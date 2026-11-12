# Example MySQL connection

## Step 1

Install and configure the module (see general README.md)

## Step 2

Apply `schema.ti` in the created `//schema` collection.

## Step 3

Apply `stuff.ti` in the `//stuff` collection.

## Step 4

Test the MySQL endpoint:
```bash
mysql -h 127.0.0.1 -P 3306 -u iris -p
```

Next try some queries:
```
Enter password: siri

mysql> use example;
Database changed

mysql> select * from articles;
+------+----------+-----------------------+----------+-------+
| id   | name     | description           | supplier | price |
+------+----------+-----------------------+----------+-------+
| a001 | Article1 | NULL                  | s001     |   2.5 |
| a002 | Article2 | This is a description | s001     |     4 |
| a003 | Article3 | NULL                  | s002     |  99.1 |
| a004 | Article4 | NULL                  | s002     |   2.8 |
+------+----------+-----------------------+----------+-------+
4 rows in set (0.00 sec)

mysql>
```

### Schema

```json
{
  "database": "example",
  "schema_version": 1,
  "tables": [
    {
      "columns": [
        {
          "comment": null,
          "name": "id",
          "nullable": false,
          "primary": true,
          "type": "TinyText"
        },
        {
          "comment": null,
          "name": "name",
          "nullable": false,
          "primary": false,
          "type": "Text"
        },
        {
          "comment": null,
          "name": "location",
          "nullable": true,
          "primary": false,
          "type": "Text"
        }
      ],
      "comment": "Suppliers from stuff",
      "name": "suppliers",
      "scope": "//stuff",
      "select": {
        "all": "get_suppliers();",
        "count": "get_suppliers().len();",
        "filter": "get_suppliers().filter(closure(filter));"
      }
    },
    {
      "columns": [
        {
          "comment": null,
          "name": "id",
          "nullable": false,
          "primary": true,
          "type": "TinyText"
        },
        {
          "comment": null,
          "name": "name",
          "nullable": false,
          "primary": false,
          "type": "Text"
        },
        {
          "comment": null,
          "name": "description",
          "nullable": true,
          "primary": false,
          "type": "Text"
        },
        {
          "comment": null,
          "name": "supplier",
          "nullable": false,
          "primary": false,
          "type": "TinyText"
        },
        {
          "comment": null,
          "name": "price",
          "nullable": false,
          "primary": false,
          "type": "Float32"
        }
      ],
      "comment": "Articles from stuff",
      "name": "articles",
      "scope": "//stuff",
      "select": {
        "all": "get_articles();",
        "count": "get_articles().len();",
        "filter": "get_articles().filter(closure(filter));"
      }
    }
  ],
  "users": [
    {
      "password": "siri",
      "username": "iris"
    }
  ],
  "version": 1
}
```