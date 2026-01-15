# FlashDB Commands Reference

## String Commands

### SET key value
Set key to hold the string value. If key already holds a value, it is overwritten.

**Time complexity:** O(1)

**Return value:** Simple string reply: OK

**Example:**
```
SET mykey "Hello"
```

---

### GET key
Get the value of key. If the key does not exist, nil is returned.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the value of key, or nil when key does not exist.

**Example:**
```
GET mykey
```

---

### DEL key [key ...]
Removes the specified keys. A key is ignored if it does not exist.

**Time complexity:** O(N) where N is the number of keys

**Return value:** Integer reply: The number of keys that were removed.

**Example:**
```
DEL key1 key2 key3
```

---

## Key Commands

### EXISTS key [key ...]
Returns if key exists.

**Time complexity:** O(N) where N is the number of keys

**Return value:** Integer reply: The number of keys that exist.

**Example:**
```
EXISTS key1 key2
```

---

### KEYS pattern
Returns all keys matching pattern. Only `*` pattern is supported (returns all keys).

**Time complexity:** O(N)

**Return value:** Array reply: list of keys matching pattern.

**Example:**
```
KEYS *
```

---

## Server Commands

### PING
Returns PONG. This command is useful for testing connection.

**Time complexity:** O(1)

**Return value:** Simple string reply: PONG

**Example:**
```
PING
```

---

### DBSIZE
Return the number of keys in the currently-selected database.

**Time complexity:** O(1)

**Return value:** Integer reply: number of keys

**Example:**
```
DBSIZE
```

---

### FLUSHDB
Delete all the keys of the currently selected DB. This command never fails.

**Time complexity:** O(N)

**Return value:** Simple string reply: OK

**Example:**
```
FLUSHDB
```

---

## Sorted Set Commands

### ZADD key score member [score member ...]
Adds all the specified members with the specified scores to the sorted set.

**Time complexity:** O(log(N)) for each item added

**Return value:** Integer reply: number of elements added

**Example:**
```
ZADD myzset 1 "one" 2 "two" 3 "three"
```

---

### ZSCORE key member
Returns the score of member in the sorted set at key.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the score of member

**Example:**
```
ZSCORE myzset "one"
```

---

### ZRANK key member
Returns the rank of member in the sorted set at key, with scores ordered low to high.

**Time complexity:** O(log(N))

**Return value:** Integer reply: the rank of member

**Example:**
```
ZRANK myzset "one"
```

---

### ZREVRANK key member
Returns the rank of member in the sorted set at key, with scores ordered high to low.

**Time complexity:** O(log(N))

**Return value:** Integer reply: the rank of member

**Example:**
```
ZREVRANK myzset "three"
```

---

### ZRANGE key start stop [WITHSCORES]
Returns the specified range of elements in the sorted set at key.

**Time complexity:** O(log(N)+M)

**Return value:** Array reply: list of elements in the specified range

**Example:**
```
ZRANGE myzset 0 -1 WITHSCORES
```

---

### ZREVRANGE key start stop [WITHSCORES]
Returns the specified range of elements in reverse order.

**Time complexity:** O(log(N)+M)

**Return value:** Array reply: list of elements in the specified range

**Example:**
```
ZREVRANGE myzset 0 -1 WITHSCORES
```

---

### ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
Returns all elements in the sorted set at key with a score between min and max.

**Time complexity:** O(log(N)+M)

**Return value:** Array reply: list of elements in the specified score range

**Example:**
```
ZRANGEBYSCORE myzset -inf +inf
```

---

### ZREM key member [member ...]
Removes the specified members from the sorted set.

**Time complexity:** O(M*log(N))

**Return value:** Integer reply: number of members removed

**Example:**
```
ZREM myzset "one"
```

---

### ZINCRBY key increment member
Increments the score of member in the sorted set by increment.

**Time complexity:** O(log(N))

**Return value:** Bulk string reply: the new score of member

**Example:**
```
ZINCRBY myzset 2 "one"
```

---

### ZCARD key
Returns the cardinality (number of elements) of the sorted set.

**Time complexity:** O(1)

**Return value:** Integer reply: the cardinality of the sorted set

**Example:**
```
ZCARD myzset
```

---

### ZCOUNT key min max
Returns the number of elements in the sorted set with a score between min and max.

**Time complexity:** O(log(N))

**Return value:** Integer reply: the number of elements in the specified score range

**Example:**
```
ZCOUNT myzset 1 3
```

---

### ZPOPMIN key [count]
Removes and returns the members with the lowest scores.

**Time complexity:** O(log(N)*M)

**Return value:** Array reply: list of popped elements and scores

**Example:**
```
ZPOPMIN myzset 2
```

---

### ZPOPMAX key [count]
Removes and returns the members with the highest scores.

**Time complexity:** O(log(N)*M)

**Return value:** Array reply: list of popped elements and scores

**Example:**
```
ZPOPMAX myzset 2
```

---

## Transaction Commands

### MULTI
Marks the start of a transaction block.

**Return value:** Simple string reply: OK

**Example:**
```
MULTI
SET key1 "value1"
SET key2 "value2"
EXEC
```

---

### EXEC
Executes all commands issued after MULTI.

**Return value:** Array reply: results of each command

---

### DISCARD
Flushes all previously queued commands and restores connection state to normal.

**Return value:** Simple string reply: OK

---

## Pub/Sub Commands

### SUBSCRIBE channel [channel ...]
Subscribes the client to the specified channels.

**Example:**
```
SUBSCRIBE news updates
```

---

### UNSUBSCRIBE [channel ...]
Unsubscribes the client from the given channels.

**Example:**
```
UNSUBSCRIBE news
```

---

### PUBLISH channel message
Posts a message to the given channel.

**Return value:** Integer reply: number of clients that received the message

**Example:**
```
PUBLISH news "Hello World"
```

---

## Authentication Commands

### AUTH password
Authenticates to the server.

**Return value:** Simple string reply: OK on success, Error on failure

**Example:**
```
AUTH mypassword
```
