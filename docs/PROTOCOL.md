# RESP Protocol Specification

## Overview

FlashDB uses RESP (Redis Serialization Protocol) for client-server communication. This is a subset implementation supporting the most common data types.

## Data Types

### Simple Strings
Prefix: `+`
```
+OK\r\n
```

### Errors
Prefix: `-`
```
-ERR unknown command\r\n
```

### Integers
Prefix: `:`
```
:1000\r\n
```

### Bulk Strings
Prefix: `$`
```
$5\r\nhello\r\n
```

Null bulk string:
```
$-1\r\n
```

### Arrays
Prefix: `*`
```
*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n
```

Null array:
```
*-1\r\n
```

## Supported Commands

### PING
Test connection.
```
Request:  *1\r\n$4\r\nPING\r\n
Response: +PONG\r\n
```

### SET key value
Set a key-value pair.
```
Request:  *3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n
Response: +OK\r\n
```

### GET key
Get value by key.
```
Request:  *2\r\n$3\r\nGET\r\n$3\r\nkey\r\n
Response: $5\r\nvalue\r\n
```

If key doesn't exist:
```
Response: $-1\r\n
```

### DEL key [key ...]
Delete one or more keys.
```
Request:  *2\r\n$3\r\nDEL\r\n$3\r\nkey\r\n
Response: :1\r\n
```

Response is the number of keys deleted.

### EXISTS key [key ...]
Check if keys exist.
```
Request:  *2\r\n$6\r\nEXISTS\r\n$3\r\nkey\r\n
Response: :1\r\n
```

Response is the number of keys that exist.

### KEYS pattern
Find all keys matching pattern (only `*` supported for all keys).
```
Request:  *2\r\n$4\r\nKEYS\r\n$1\r\n*\r\n
Response: *2\r\n$4\r\nkey1\r\n$4\r\nkey2\r\n
```

### DBSIZE
Return number of keys.
```
Request:  *1\r\n$6\r\nDBSIZE\r\n
Response: :100\r\n
```

### FLUSHDB
Delete all keys.
```
Request:  *1\r\n$7\r\nFLUSHDB\r\n
Response: +OK\r\n
```

## Error Responses

### Unknown Command
```
-ERR unknown command 'FOO'\r\n
```

### Wrong Number of Arguments
```
-ERR wrong number of arguments for 'GET' command\r\n
```

### Internal Error
```
-ERR internal error\r\n
```
